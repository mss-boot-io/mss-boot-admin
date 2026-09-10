package litellmops

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
	"gorm.io/gorm"
)

type budgetGateway struct {
	mu                    sync.Mutex
	budget                *float64
	keys                  map[string]*float64
	userWrites            int
	writtenBudgets        []float64
	keyWrites             map[string]int
	breakUserResponseOnce bool
	ignoreUserWrite       bool
	userWriteStatusOnce   int
	applyBeforeFailure    bool
	userWriteStarted      chan struct{}
	continueUserWrite     chan struct{}
	userWriteStartOnce    sync.Once
	failKeyOnce           string
	writeDelay            time.Duration
}

func newBudgetGateway(budget *float64) *budgetGateway {
	return &budgetGateway{budget: budget, keys: map[string]*float64{}, keyWrites: map[string]int{}}
}

func (gateway *budgetGateway) server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-master-key" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/user/info":
			gateway.mu.Lock()
			budget := gateway.budget
			gateway.mu.Unlock()
			_ = json.NewEncoder(writer).Encode(map[string]any{"user_info": map[string]any{"user_id": "u-1", "user_email": "buyer@example.com", "max_budget": budget}})
		case "/user/update":
			var body map[string]any
			_ = json.NewDecoder(request.Body).Decode(&body)
			if gateway.writeDelay > 0 {
				time.Sleep(gateway.writeDelay)
			}
			value := body["max_budget"].(float64)
			gateway.mu.Lock()
			gateway.userWrites++
			gateway.writtenBudgets = append(gateway.writtenBudgets, value)
			status := gateway.userWriteStatusOnce
			gateway.userWriteStatusOnce = 0
			if !gateway.ignoreUserWrite && (status == 0 || gateway.applyBeforeFailure) {
				gateway.budget = &value
			}
			broken := gateway.breakUserResponseOnce
			gateway.breakUserResponseOnce = false
			gateway.mu.Unlock()
			if gateway.userWriteStarted != nil {
				gateway.userWriteStartOnce.Do(func() { close(gateway.userWriteStarted) })
			}
			if gateway.continueUserWrite != nil {
				<-gateway.continueUserWrite
			}
			if status != 0 {
				writer.WriteHeader(status)
				return
			}
			if broken {
				writer.Header().Set("Content-Length", "100")
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write([]byte("{"))
				return
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"status": "ok"})
		case "/user/list":
			gateway.mu.Lock()
			budget := gateway.budget
			gateway.mu.Unlock()
			_ = json.NewEncoder(writer).Encode(map[string]any{"users": []any{map[string]any{"user_id": "u-1", "user_email": "buyer@example.com", "max_budget": budget}}})
		case "/key/list":
			gateway.mu.Lock()
			items := make([]map[string]any, 0, len(gateway.keys))
			for token, value := range gateway.keys {
				items = append(items, map[string]any{"token": token, "key_alias": token, "user_id": "u-1", "max_budget": value})
			}
			gateway.mu.Unlock()
			_ = json.NewEncoder(writer).Encode(map[string]any{"keys": items})
		case "/key/update":
			var body map[string]any
			_ = json.NewDecoder(request.Body).Decode(&body)
			token := body["key"].(string)
			value := body["max_budget"].(float64)
			gateway.mu.Lock()
			gateway.keyWrites[token]++
			fail := gateway.failKeyOnce == token
			if fail {
				gateway.failKeyOnce = ""
			} else {
				gateway.keys[token] = &value
			}
			gateway.mu.Unlock()
			if fail {
				writer.WriteHeader(http.StatusBadGateway)
				return
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"status": "ok"})
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
}

func seedRechargeUser(t *testing.T, db *gorm.DB) UserSnapshot {
	t.Helper()
	now := time.Now().UTC()
	snapshot := UserSnapshot{ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now, UserID: "u-1", Email: "buyer@example.com", Models: "[]", SyncedAt: now}
	if err := db.Create(&snapshot).Error; err != nil {
		t.Fatalf("seed user snapshot: %v", err)
	}
	return snapshot
}

func testClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	client, err := NewClient(server.URL, "test-master-key", server.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func withImmediateConfirmation(t *testing.T) {
	t.Helper()
	previous := rechargeConfirmDelays
	rechargeConfirmDelays = []time.Duration{0}
	t.Cleanup(func() { rechargeConfirmDelays = previous })
}

func withImmediateUncertainRetry(t *testing.T) {
	t.Helper()
	previousRetry := rechargeUncertainRetryAfter
	previousStable := rechargeStableObservationWindow
	rechargeUncertainRetryAfter = 0
	rechargeStableObservationWindow = 0
	t.Cleanup(func() {
		rechargeUncertainRetryAfter = previousRetry
		rechargeStableObservationWindow = previousStable
	})
}

func TestRechargeUnknownResponseReadbackAndReplay(t *testing.T) {
	withImmediateConfirmation(t)
	db := openTestDB(t)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	gateway.breakUserResponseOnce = true
	server := gateway.server(t)
	t.Cleanup(server.Close)

	request := RechargeRequest{AmountUSDMicro: 5_000_000, IdempotencyKey: "unknown-response", Reason: "paid order"}
	first, err := ApplyRecharge(context.Background(), db, testClient(t, server), snapshot, request, "operator-a")
	if err != nil || first.Status != RechargeCompleted {
		t.Fatalf("unknown response should converge by readback: record=%+v err=%v", first, err)
	}
	replay, err := ApplyRecharge(context.Background(), db, testClient(t, server), snapshot, request, "operator-a")
	if err != nil || replay.ID != first.ID {
		t.Fatalf("replay should return original: record=%+v err=%v", replay, err)
	}
	gateway.mu.Lock()
	writes := gateway.userWrites
	gateway.mu.Unlock()
	if writes != 1 {
		t.Fatalf("expected one upstream write, got %d", writes)
	}
	if _, err := ApplyRecharge(context.Background(), db, testClient(t, server), snapshot, RechargeRequest{AmountUSDMicro: 6_000_000, IdempotencyKey: request.IdempotencyKey}, "operator-a"); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("payload conflict must be rejected: %v", err)
	}
}

func TestRechargePendingBarrierSurvivesUntilAuthoritativeReadback(t *testing.T) {
	withImmediateConfirmation(t)
	db := openTestDB(t)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	gateway.ignoreUserWrite = true
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)

	first, err := ApplyRecharge(context.Background(), db, client, snapshot, RechargeRequest{AmountUSDMicro: 5_000_000, IdempotencyKey: "pending-first"}, "operator-a")
	if !errors.Is(err, ErrReconcileRequired) || first == nil || first.Status != RechargeAppliedUnverified {
		t.Fatalf("first command should remain uncertain: record=%+v err=%v", first, err)
	}
	_, err = ApplyRecharge(context.Background(), db, client, snapshot, RechargeRequest{AmountUSDMicro: 2_000_000, IdempotencyKey: "pending-second"}, "operator-a")
	if !errors.Is(err, ErrPendingRecharge) {
		t.Fatalf("different command must be fenced: %v", err)
	}
	managementCalled := false
	if _, err := withUserRechargeFence(context.Background(), db, snapshot.UserID, func() (any, error) {
		managementCalled = true
		return nil, nil
	}); !errors.Is(err, ErrPendingRecharge) || managementCalled {
		t.Fatalf("management mutation bypassed pending recharge: called=%v err=%v", managementCalled, err)
	}
	gateway.mu.Lock()
	if gateway.userWrites != 1 {
		t.Fatalf("fenced command wrote upstream: %d", gateway.userWrites)
	}
	visible := 15.0
	gateway.budget = &visible
	gateway.ignoreUserWrite = false
	gateway.mu.Unlock()
	resolved, err := ReconcileRecharge(context.Background(), db, client, first.ID)
	if err != nil || resolved.Status != RechargeCompleted {
		t.Fatalf("authoritative readback should resolve: record=%+v err=%v", resolved, err)
	}
	second, err := ApplyRecharge(context.Background(), db, client, snapshot, RechargeRequest{AmountUSDMicro: 2_000_000, IdempotencyKey: "pending-second"}, "operator-a")
	if err != nil || second.Status != RechargeCompleted {
		t.Fatalf("next command should run after resolution: record=%+v err=%v", second, err)
	}
}

func TestRechargeFiveHundredResponseNeedsStableReadBeforeAbsoluteRetry(t *testing.T) {
	withImmediateConfirmation(t)
	withImmediateUncertainRetry(t)
	db := openTestDB(t)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	gateway.userWriteStatusOnce = http.StatusServiceUnavailable
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)

	record, err := ApplyRecharge(context.Background(), db, client, snapshot, RechargeRequest{AmountUSDMicro: 5_000_000, IdempotencyKey: "five-hundred"}, "operator-a")
	if !upstreamResultUncertain(err) || record.Status != RechargeAppliedUnverified {
		t.Fatalf("5xx must be uncertain: record=%+v err=%v", record, err)
	}
	if _, err := ReconcileRecharge(context.Background(), db, client, record.ID); !errors.Is(err, ErrReconcileRequired) {
		t.Fatalf("first stable observation must remain read-only: %v", err)
	}
	gateway.mu.Lock()
	firstPassWrites := gateway.userWrites
	gateway.mu.Unlock()
	if firstPassWrites != 1 {
		t.Fatalf("first reconcile blindly rewrote: writes=%d", firstPassWrites)
	}
	resolved, err := ReconcileRecharge(context.Background(), db, client, record.ID)
	if err != nil || resolved.Status != RechargeCompleted {
		t.Fatalf("stable absolute retry should converge: record=%+v err=%v", resolved, err)
	}
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	if gateway.userWrites != 2 || *gateway.budget != 15 {
		t.Fatalf("absolute retry mismatch: writes=%d budget=%v", gateway.userWrites, *gateway.budget)
	}
}

func TestRechargeStableObservationKeepsFirstTimestamp(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db)
	now := time.Now().UTC()
	record := RechargeRecord{
		ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now, UserID: "u-observe", Amount: 1,
		AmountUSDMicro: 1_000_000, KeysUpdated: "[]", Operator: "operator-a", IdempotencyKey: "observe",
		PayloadHash: rechargePayloadHash("u-observe", 1_000_000, false), Source: "manual",
		Status: RechargeAppliedUnverified, Version: 1,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("create observation record: %v", err)
	}
	previous := rechargeStableObservationWindow
	rechargeStableObservationWindow = 60 * time.Millisecond
	t.Cleanup(func() { rechargeStableObservationWindow = previous })
	stable, err := observeRechargeBudget(context.Background(), db, &record, 10_000_000)
	if err != nil || stable || record.LastObservedAt == nil {
		t.Fatalf("first observation: stable=%v record=%+v err=%v", stable, record, err)
	}
	firstObservedAt := *record.LastObservedAt
	time.Sleep(20 * time.Millisecond)
	stable, err = observeRechargeBudget(context.Background(), db, &record, 10_000_000)
	if err != nil || stable || record.LastObservedAt == nil || !record.LastObservedAt.Equal(firstObservedAt) {
		t.Fatalf("fast observation must preserve first timestamp: stable=%v record=%+v err=%v", stable, record, err)
	}
	time.Sleep(50 * time.Millisecond)
	stable, err = observeRechargeBudget(context.Background(), db, &record, 10_000_000)
	if err != nil || !stable {
		t.Fatalf("delayed observation should become stable: stable=%v err=%v", stable, err)
	}
}

func TestRechargeUnlimitedBudgetRejectedBeforeWrite(t *testing.T) {
	withImmediateConfirmation(t)
	db := openTestDB(t)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	gateway := newBudgetGateway(nil)
	server := gateway.server(t)
	t.Cleanup(server.Close)
	record, err := ApplyRecharge(context.Background(), db, testClient(t, server), snapshot, RechargeRequest{AmountUSDMicro: 1_000_000, IdempotencyKey: "unlimited"}, "operator-a")
	if !errors.Is(err, ErrUnlimitedBudget) || record == nil || record.Status != RechargeTerminalFailed {
		t.Fatalf("unexpected unlimited result: record=%+v err=%v", record, err)
	}
	if gateway.userWrites != 0 {
		t.Fatalf("unlimited user must not be written")
	}
}

func TestRechargeConcurrentReplayWritesOnce(t *testing.T) {
	withImmediateConfirmation(t)
	db := openTestDB(t)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	gateway.writeDelay = 50 * time.Millisecond
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)
	request := RechargeRequest{AmountUSDMicro: 5_000_000, IdempotencyKey: "concurrent"}
	start := make(chan struct{})
	var successes atomic.Int32
	var wait sync.WaitGroup
	for index := 0; index < 8; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			record, err := ApplyRecharge(context.Background(), db, client, snapshot, request, "operator-a")
			if err == nil && record.Status == RechargeCompleted {
				successes.Add(1)
			}
			if err != nil && !errors.Is(err, ErrRechargeBusy) {
				t.Errorf("unexpected concurrent error: %v", err)
			}
		}()
	}
	close(start)
	wait.Wait()
	if successes.Load() == 0 {
		t.Fatal("at least one caller must complete")
	}
	gateway.mu.Lock()
	writes := gateway.userWrites
	finalBudget := *gateway.budget
	gateway.mu.Unlock()
	if writes != 1 || finalBudget != 15 {
		t.Fatalf("concurrent replay double-applied: writes=%d budget=%v", writes, finalBudget)
	}
}

func TestRechargeConcurrentDifferentCommandsAreSerializedWithoutLostCredit(t *testing.T) {
	withImmediateConfirmation(t)
	db := openTestDB(t)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(4)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	gateway.userWriteStarted = make(chan struct{})
	gateway.continueUserWrite = make(chan struct{})
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)
	firstRequest := RechargeRequest{AmountUSDMicro: 5_000_000, IdempotencyKey: "different-first"}
	secondRequest := RechargeRequest{AmountUSDMicro: 2_000_000, IdempotencyKey: "different-second"}
	type result struct {
		record *RechargeRecord
		err    error
	}
	firstDone := make(chan result, 1)
	go func() {
		record, err := ApplyRecharge(context.Background(), db, client, snapshot, firstRequest, "operator-a")
		firstDone <- result{record: record, err: err}
	}()
	select {
	case <-gateway.userWriteStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first command did not reach upstream write")
	}
	if _, err := ApplyRecharge(context.Background(), db, client, snapshot, secondRequest, "operator-b"); !errors.Is(err, ErrRechargeBusy) && !errors.Is(err, ErrPendingRecharge) {
		t.Fatalf("second command must be fenced while first writes: %v", err)
	}
	close(gateway.continueUserWrite)
	first := <-firstDone
	if first.err != nil || first.record == nil || first.record.Status != RechargeCompleted {
		t.Fatalf("first command failed: record=%+v err=%v", first.record, first.err)
	}
	second, err := ApplyRecharge(context.Background(), db, client, snapshot, secondRequest, "operator-b")
	if err != nil || second.Status != RechargeCompleted {
		t.Fatalf("second command retry failed: record=%+v err=%v", second, err)
	}
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	if gateway.userWrites != 2 || *gateway.budget != 17 || len(gateway.writtenBudgets) != 2 || gateway.writtenBudgets[0] != 15 || gateway.writtenBudgets[1] != 17 {
		t.Fatalf("serialized commands lost or lowered credit: writes=%d budget=%v targets=%v", gateway.userWrites, *gateway.budget, gateway.writtenBudgets)
	}
}

func TestManagementBudgetMutationAndRechargeShareUserFence(t *testing.T) {
	withImmediateConfirmation(t)
	db := openTestDB(t)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	gateway.userWriteStarted = make(chan struct{})
	gateway.continueUserWrite = make(chan struct{})
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)
	managementDone := make(chan error, 1)
	managementBudget := 20.0
	go func() {
		_, err := withUserRechargeFence(context.Background(), db, snapshot.UserID, func() (any, error) {
			return client.updateUser(context.Background(), snapshot.UserID, userMutationRequest{MaxBudget: &managementBudget})
		})
		managementDone <- err
	}()
	select {
	case <-gateway.userWriteStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("management mutation did not reach upstream")
	}
	request := RechargeRequest{AmountUSDMicro: 5_000_000, IdempotencyKey: "after-management"}
	if _, err := ApplyRecharge(context.Background(), db, client, snapshot, request, "operator-b"); !errors.Is(err, ErrRechargeBusy) {
		t.Fatalf("recharge must not race management write: %v", err)
	}
	close(gateway.continueUserWrite)
	if err := <-managementDone; err != nil {
		t.Fatalf("management mutation failed: %v", err)
	}
	record, err := ApplyRecharge(context.Background(), db, client, snapshot, request, "operator-b")
	if err != nil || record.Status != RechargeCompleted {
		t.Fatalf("recharge after management mutation failed: record=%+v err=%v", record, err)
	}
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	if *gateway.budget != 25 || len(gateway.writtenBudgets) != 2 || gateway.writtenBudgets[0] != 20 || gateway.writtenBudgets[1] != 25 {
		t.Fatalf("cross-workflow write lowered or lost budget: budget=%v targets=%v", *gateway.budget, gateway.writtenBudgets)
	}
}

func TestRechargePartialKeyFailureReconcilesAbsoluteTargets(t *testing.T) {
	withImmediateConfirmation(t)
	withImmediateUncertainRetry(t)
	db := openTestDB(t)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	budget, keyBudgetA, keyBudgetB := 10.0, 10.0, 10.0
	gateway := newBudgetGateway(&budget)
	gateway.keys["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"] = &keyBudgetA
	gateway.keys["bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"] = &keyBudgetB
	gateway.failKeyOnce = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)
	record, err := ApplyRecharge(context.Background(), db, client, snapshot, RechargeRequest{AmountUSDMicro: 5_000_000, RaiseKeys: true, IdempotencyKey: "partial-keys"}, "operator-a")
	if !errors.Is(err, ErrReconcileRequired) || record.Status != RechargeAppliedUnverified {
		t.Fatalf("partial key update must be unverified: record=%+v err=%v", record, err)
	}
	reconciled, err := ReconcileRecharge(context.Background(), db, client, record.ID)
	if !errors.Is(err, ErrReconcileRequired) || reconciled.Status != RechargeAppliedUnverified {
		t.Fatalf("unknown key result must stay readback-only: record=%+v err=%v", reconciled, err)
	}
	reconciled, err = ReconcileRecharge(context.Background(), db, client, record.ID)
	if err != nil || reconciled.Status != RechargeCompleted {
		t.Fatalf("stable absolute key retry failed: record=%+v err=%v", reconciled, err)
	}
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	if gateway.userWrites != 1 || *gateway.budget != 15 || *gateway.keys["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"] != 15 || *gateway.keys["bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"] != 15 {
		t.Fatalf("unexpected reconciled state: userWrites=%d budget=%v keys=%v", gateway.userWrites, *gateway.budget, gateway.keys)
	}
	if gateway.keyWrites["bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"] != 2 {
		t.Fatalf("expected exactly one bounded absolute key retry: %v", gateway.keyWrites)
	}
}

func TestSalesManualOrderLifecycleAndConflict(t *testing.T) {
	withImmediateConfirmation(t)
	db := openTestDB(t)
	migrateTestDB(t, db)
	seedRechargeUser(t, db)
	now := time.Now().UTC()
	product := SalesProduct{ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now, Channel: "xianyu", Shop: "shop-a", ExternalItemID: "item-a", SKU: "", Title: "credit", PriceCNYFen: 1000, CreditUSDMicro: 5_000_000, RaiseKeys: false, Enabled: true, Version: 1}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	request := orderCreateRequest{Channel: "xianyu", Shop: "shop-a", ExternalOrderID: "order-a", AdjustmentType: "purchase", ProductID: product.ID, UserEmail: "buyer@example.com", PaidCNYFen: 1000, PaymentStatus: "paid"}
	order, created, err := createSalesOrder(context.Background(), db, request, "operator-a", false)
	if err != nil || !created || order.AdjustmentType != "credit" || order.Status != OrderReceived {
		t.Fatalf("create order: %+v created=%v err=%v", order, created, err)
	}
	replay, created, err := createSalesOrder(context.Background(), db, request, "operator-a", false)
	if err != nil || created || replay.ID != order.ID {
		t.Fatalf("order replay: %+v created=%v err=%v", replay, created, err)
	}
	conflict := request
	conflict.PaidCNYFen = 999
	if _, _, err := createSalesOrder(context.Background(), db, conflict, "operator-a", false); !errors.Is(err, ErrOrderConflict) {
		t.Fatalf("expected order conflict, got %v", err)
	}
	if err := verifySalesOrder(context.Background(), db, order, "verifier-b"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := matchSalesOrder(context.Background(), db, order, orderMatchRequest{ProductID: product.ID, UserEmail: "buyer@example.com"}, "matcher-b"); err != nil {
		t.Fatalf("match: %v", err)
	}
	if err := approveSalesOrder(context.Background(), db, order, "approver-b"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	server := gateway.server(t)
	t.Cleanup(server.Close)
	if err := executeSalesOrder(context.Background(), db, testClient(t, server), order, "executor-c"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if order.Status != OrderCompleted || order.RechargeID == "" {
		t.Fatalf("unexpected completed order: %+v", order)
	}
	if *gateway.budget != 15 || gateway.userWrites != 1 {
		t.Fatalf("unexpected applied budget: %v writes=%d", *gateway.budget, gateway.userWrites)
	}
}

func TestOrderExecutionLeaseFencesFreshAndStaleWorkers(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db)
	now := time.Now().UTC()
	freshUntil := now.Add(time.Minute)
	fresh := SalesOrder{
		ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now, Channel: "xianyu", Shop: "shop-a",
		ExternalOrderID: "fresh-order", AdjustmentType: "credit", PayloadHash: "fresh-hash",
		PaidCNYFen: 1000, PaymentStatus: "paid", SourceTrust: "manual", Status: OrderExecuting,
		Operator: "operator-a", Version: 1, ExecutionHolder: "old-holder", ExecutionFence: 7,
		ExecutionLeaseUntil: &freshUntil,
	}
	if err := db.Create(&fresh).Error; err != nil {
		t.Fatalf("create fresh executing order: %v", err)
	}
	if _, _, err := claimOrderExecution(context.Background(), db, &fresh, []string{OrderExecuting}, "operator-b", "reconcile_start"); !errors.Is(err, ErrOrderBusy) {
		t.Fatalf("fresh executing order must remain busy: %v", err)
	}

	staleUntil := now.Add(-time.Minute)
	stale := SalesOrder{
		ID: newSnapshotID(), CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour), Channel: "xianyu", Shop: "shop-a",
		ExternalOrderID: "stale-order", AdjustmentType: "credit", PayloadHash: "stale-hash",
		PaidCNYFen: 1000, PaymentStatus: "paid", SourceTrust: "manual", Status: OrderExecuting,
		Operator: "operator-a", Version: 1, ExecutionHolder: "stale-holder", ExecutionFence: 3,
		ExecutionLeaseUntil: &staleUntil,
	}
	if err := db.Create(&stale).Error; err != nil {
		t.Fatalf("create stale executing order: %v", err)
	}
	oldWorker := stale
	holder, fence, err := claimOrderExecution(context.Background(), db, &stale, []string{OrderExecuting}, "operator-b", "reconcile_start")
	if err != nil || holder == "" || fence != 4 {
		t.Fatalf("stale order takeover failed: holder=%q fence=%d err=%v", holder, fence, err)
	}
	if err := updateClaimedOrder(context.Background(), db, &oldWorker, "stale-holder", 3, map[string]any{"status": OrderCompleted}, true, "old_finish", "completed", "operator-a"); !errors.Is(err, ErrOrderBusy) {
		t.Fatalf("old worker must be fenced: %v", err)
	}
	if err := updateClaimedOrder(context.Background(), db, &stale, holder, fence, map[string]any{"status": OrderRetryableFailed}, true, "release", "retryable", "operator-b"); err != nil {
		t.Fatalf("new holder should update order: %v", err)
	}
}

func TestStaleExecutingOrderRecoversRechargeBySourceReference(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db)
	now := time.Now().UTC()
	staleUntil := now.Add(-time.Minute)
	order := SalesOrder{
		ID: newSnapshotID(), CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour), Channel: "xianyu", Shop: "shop-a",
		ExternalOrderID: "crash-window", AdjustmentType: "credit", PayloadHash: "crash-hash", ProductID: "product-a",
		UserID: "u-1", UserEmail: "buyer@example.com", PaidCNYFen: 1000, CreditUSDMicro: 5_000_000,
		PaymentStatus: "paid", SourceTrust: "manual", Status: OrderExecuting, Operator: "operator-a", Version: 1,
		ExecutionHolder: "dead-worker", ExecutionFence: 1, ExecutionLeaseUntil: &staleUntil,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create crash order: %v", err)
	}
	before, target := int64(10_000_000), int64(15_000_000)
	recharge := RechargeRecord{
		ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now, CompletedAt: &now, UserID: "u-1",
		Amount: 5, BeforeBudget: 10, AfterBudget: 15, AmountUSDMicro: 5_000_000,
		BeforeBudgetUSDMicro: &before, TargetAfterUSDMicro: &target, KeysUpdated: "[]", Operator: "operator-a",
		IdempotencyKey: "sales-order:" + order.ID + ":credit-v1", PayloadHash: rechargePayloadHash("u-1", 5_000_000, false),
		Source: "sales_order", SourceRef: order.ID, Status: RechargeCompleted, Version: 1,
	}
	if err := db.Create(&recharge).Error; err != nil {
		t.Fatalf("create completed recharge: %v", err)
	}
	if err := reconcileSalesOrder(context.Background(), db, nil, &order, "operator-b"); err != nil {
		t.Fatalf("recover crash window: %v", err)
	}
	if order.Status != OrderCompleted || order.RechargeID != recharge.ID {
		t.Fatalf("order did not recover unique recharge: %+v", order)
	}
	if containsOrderStatus(OrderAppliedUnverified, refundReviewAllowedStatuses()) || containsOrderStatus(OrderReconcileRequired, refundReviewAllowedStatuses()) || containsOrderStatus(OrderRetryableFailed, refundReviewAllowedStatuses()) {
		t.Fatal("pending recharge order states must not enter refund review")
	}
}

func TestRetryableOrderWithoutRechargeRecoversThroughReconcile(t *testing.T) {
	withImmediateConfirmation(t)
	db := openTestDB(t)
	migrateTestDB(t, db)
	seedRechargeUser(t, db)
	now := time.Now().UTC()
	order := SalesOrder{
		ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now, Channel: "xianyu", Shop: "shop-a",
		ExternalOrderID: "blocked-then-reconcile", AdjustmentType: "credit", PayloadHash: "blocked-hash", ProductID: "product-a",
		UserID: "u-1", UserEmail: "buyer@example.com", PaidCNYFen: 1000, CreditUSDMicro: 5_000_000,
		PaymentStatus: "paid", SourceTrust: "manual", Status: OrderApproved, Operator: "operator-a", Approver: "approver-b", Version: 1,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create approved order: %v", err)
	}
	pending := RechargeRecord{
		ID: newSnapshotID(), CreatedAt: now.Add(-time.Minute), UpdatedAt: now, UserID: "u-1", Amount: 1,
		AmountUSDMicro: 1_000_000, KeysUpdated: "[]", Operator: "operator-a", IdempotencyKey: "earlier-pending",
		PayloadHash: rechargePayloadHash("u-1", 1_000_000, false), Source: "manual", Status: RechargeApproved, Version: 1,
	}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatalf("create pending recharge: %v", err)
	}
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)
	if err := executeSalesOrder(context.Background(), db, client, &order, "executor-c"); !errors.Is(err, ErrPendingRecharge) {
		t.Fatalf("first execute should be blocked by earlier recharge: order=%+v err=%v", order, err)
	}
	if order.Status != OrderRetryableFailed || order.RechargeID != "" || gateway.userWrites != 0 {
		t.Fatalf("blocked order must be retryable without remote write: order=%+v writes=%d", order, gateway.userWrites)
	}
	if err := db.Model(&RechargeRecord{}).Where("id = ? AND version = ?", pending.ID, pending.Version).
		Updates(map[string]any{"status": RechargeCompleted, "completed_at": now, "version": gorm.Expr("version + 1")}).Error; err != nil {
		t.Fatalf("resolve earlier recharge: %v", err)
	}
	if err := reconcileSalesOrder(context.Background(), db, client, &order, "executor-c"); err != nil {
		t.Fatalf("UI reconcile should recover order: order=%+v err=%v", order, err)
	}
	if order.Status != OrderCompleted || order.RechargeID == "" || gateway.userWrites != 1 || *gateway.budget != 15 {
		t.Fatalf("recovered order mismatch: order=%+v writes=%d budget=%v", order, gateway.userWrites, *gateway.budget)
	}
	var salesRecharges int64
	if err := db.Model(&RechargeRecord{}).Where("source = ? AND source_ref = ?", "sales_order", order.ID).Count(&salesRecharges).Error; err != nil || salesRecharges != 1 {
		t.Fatalf("recovery must reserve exactly one sales recharge: count=%d err=%v", salesRecharges, err)
	}
}

func TestOrderBlockedByManagementQuarantineRecoversThroughReconcile(t *testing.T) {
	withImmediateConfirmation(t)
	db := openTestDB(t)
	migrateTestDB(t, db)
	seedRechargeUser(t, db)
	now := time.Now().UTC()
	order := SalesOrder{
		ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now, Channel: "xianyu", Shop: "shop-a",
		ExternalOrderID: "management-blocked", AdjustmentType: "credit", PayloadHash: "management-blocked-hash",
		UserID: "u-1", UserEmail: "buyer@example.com", PaidCNYFen: 1000, CreditUSDMicro: 5_000_000,
		PaymentStatus: "paid", SourceTrust: "manual", Status: OrderApproved, Operator: "operator-a", Approver: "approver-b", Version: 1,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create approved order: %v", err)
	}
	command := ManagementCommand{
		ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now, UserID: "u-1", UserEmail: "buyer@example.com",
		TargetType: "user", TargetID: "u-1", Action: "user_update", PayloadHash: "safe-digest",
		ExpectedState: managementExpectedJSON(managementExpectedState{Kind: "user", RequiresManualOnly: true}),
		Status:        ManagementResultUnverified, RequiresManualReview: true, Operator: "operator-a", Version: 1,
	}
	if err := db.Create(&command).Error; err != nil {
		t.Fatalf("create management quarantine: %v", err)
	}
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)
	if err := executeSalesOrder(context.Background(), db, client, &order, "executor-c"); !errors.Is(err, ErrManagementPending) {
		t.Fatalf("management quarantine should block order: order=%+v err=%v", order, err)
	}
	if order.Status != OrderRetryableFailed || order.RechargeID != "" || gateway.userWrites != 0 {
		t.Fatalf("blocked order must be retryable without recharge: order=%+v writes=%d", order, gateway.userWrites)
	}
	if err := db.Model(&ManagementCommand{}).Where("id = ?", command.ID).Updates(map[string]any{
		"status": ManagementResolvedNoop, "resolved_at": now, "version": gorm.Expr("version + 1"),
	}).Error; err != nil {
		t.Fatalf("resolve management fixture: %v", err)
	}
	if err := reconcileSalesOrder(context.Background(), db, client, &order, "executor-c"); err != nil {
		t.Fatalf("reconcile should recover after management resolution: order=%+v err=%v", order, err)
	}
	if order.Status != OrderCompleted || gateway.userWrites != 1 || *gateway.budget != 15 {
		t.Fatalf("recovered order mismatch: order=%+v writes=%d budget=%v", order, gateway.userWrites, *gateway.budget)
	}
}

func TestConnectorAuthenticationFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	raw := []byte(`{"channel":"xianyu"}`)
	check := func(token string, configured bool) int {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(raw)))
		if configured {
			timestamp := strconv.FormatInt(time.Now().Unix(), 10)
			ctx.Request.Header.Set("X-LiteLLMOps-Connector-Token", token)
			ctx.Request.Header.Set("X-LiteLLMOps-Connector-Timestamp", timestamp)
			mac := hmac.New(sha256.New, []byte("connector-test-token"))
			_, _ = mac.Write([]byte(timestamp + "\n"))
			_, _ = mac.Write(raw)
			ctx.Request.Header.Set("X-LiteLLMOps-Connector-Signature", hex.EncodeToString(mac.Sum(nil)))
		}
		if authorizeConnector(ctx, raw) {
			return http.StatusOK
		}
		return recorder.Code
	}
	t.Setenv(envConnectorSharedToken, "")
	if code := check("", false); code != http.StatusServiceUnavailable {
		t.Fatalf("disabled connector code=%d", code)
	}
	t.Setenv(envConnectorSharedToken, "connector-test-token")
	t.Setenv(envConnectorAllowedSources, "xianyu:shop-a")
	if code := check("wrong", true); code != http.StatusUnauthorized {
		t.Fatalf("wrong connector code=%d", code)
	}
	if code := check("connector-test-token", true); code != http.StatusOK {
		t.Fatalf("valid connector code=%d", code)
	}
	if !connectorSourceAllowed("xianyu", "shop-a") || connectorSourceAllowed("xianyu", "shop-b") {
		t.Fatal("connector allowlist must bind both channel and shop")
	}
}

func TestOperationsMigrationUpgradesLegacyRechargeTable(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&migrationmodels.Migration{}, &models.CasbinRule{}); err != nil {
		t.Fatalf("base migration: %v", err)
	}
	if err := db.Exec(createRechargeTableDDL["sqlite"]).Error; err != nil {
		t.Fatalf("legacy table: %v", err)
	}
	now := time.Now().UTC()
	if err := db.Exec(`INSERT INTO litellmops_recharge (id,created_at,user_id,email,amount,before_budget,after_budget,raise_keys,keys_updated,operator,reason,idempotency_key,status) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, "legacy-completed", now, "u-legacy", "legacy@example.com", 1, 10, 11, false, "[]", "old-admin", "legacy", "legacy-key", "completed").Error; err != nil {
		t.Fatalf("legacy completed row: %v", err)
	}
	if err := db.Exec(`INSERT INTO litellmops_recharge (id,created_at,user_id,email,amount,before_budget,after_budget,raise_keys,keys_updated,operator,reason,idempotency_key,status) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, "legacy-failed", now, "u-legacy-2", "legacy2@example.com", 2, 20, 22, false, "[]", "old-admin", "legacy", nil, "failed").Error; err != nil {
		t.Fatalf("legacy nullable row: %v", err)
	}
	if err := migrateOperations(db, OperationsMigrationID.String()); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if err := migrateOperations(db, OperationsMigrationID.String()); err != nil {
		t.Fatalf("idempotent second upgrade: %v", err)
	}
	for _, column := range []string{"updated_at", "amount_usd_micro", "target_after_usd_micro", "payload_hash", "version"} {
		if !db.Migrator().HasColumn(&RechargeRecord{}, column) {
			t.Fatalf("missing upgraded column %s", column)
		}
	}
	var count int64
	if err := db.Model(&RechargeRecord{}).Where("id IN ?", []string{"legacy-completed", "legacy-failed"}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("legacy rows lost: count=%d err=%v", count, err)
	}
	if !db.Migrator().HasIndex(&RechargeRecord{}, "ux_litellmops_recharge_idempotency") {
		t.Fatal("missing recharge idempotency unique index")
	}
	if err := db.Exec(`INSERT INTO litellmops_recharge (id,created_at,user_id,amount,before_budget,after_budget,raise_keys,operator,idempotency_key,status) VALUES (?,?,?,?,?,?,?,?,?,?)`, "null-after-upgrade", now, "u-null", 1, 1, 2, false, "old-admin", nil, "completed").Error; err == nil {
		t.Fatal("upgraded idempotency_key must reject NULL")
	}
	if err := db.Exec(`INSERT INTO litellmops_recharge (id,created_at,user_id,amount,before_budget,after_budget,raise_keys,operator,idempotency_key,status) VALUES (?,?,?,?,?,?,?,?,?,?)`, "duplicate-after-upgrade", now, "u-legacy", 1, 10, 11, false, "old-admin", "legacy-key", "completed").Error; err == nil {
		t.Fatal("upgraded idempotency index must reject duplicates")
	}
	if err := db.Model(&migrationmodels.Migration{}).Where("version = ?", OperationsMigrationID.String()).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("migration ledger should contain one operation row: count=%d err=%v", count, err)
	}
	// Recreate the exact previous-release schema: these nullable observation
	// columns are owned only by the new forward migration.
	for _, field := range []string{"LastObservedKeyPrefix", "LastObservedKeyUSDMicro", "LastObservedKeyAt"} {
		if err := db.Migrator().DropColumn(&RechargeRecord{}, field); err != nil {
			t.Fatalf("prepare previous schema without %s: %v", field, err)
		}
	}
	if err := migrateManagementFence(db, ManagementFenceMigrationID.String()); err != nil {
		t.Fatalf("upgrade previous production schema with management fence: %v", err)
	}
	if err := migrateManagementFence(db, ManagementFenceMigrationID.String()); err != nil {
		t.Fatalf("management fence migration must be idempotent: %v", err)
	}
	if !db.Migrator().HasTable(&ManagementCommand{}) {
		t.Fatal("management fence table was not created")
	}
	for _, field := range []string{"LastObservedKeyPrefix", "LastObservedKeyUSDMicro", "LastObservedKeyAt"} {
		if !db.Migrator().HasColumn(&RechargeRecord{}, field) {
			t.Fatalf("management fence upgrade missing recharge column %s", field)
		}
	}
	if err := db.Model(&migrationmodels.Migration{}).Where("version = ?", ManagementFenceMigrationID.String()).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("management fence ledger should contain one row: count=%d err=%v", count, err)
	}
	if err := db.Model(&models.CasbinRule{}).Where("v2 = ? AND v3 = ?", "/admin/api/litellmops/management/commands/:id/resolve", "POST").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("management resolve policy must be seeded once: count=%d err=%v", count, err)
	}
	if err := db.Model(&RechargeRecord{}).Where("id IN ?", []string{"legacy-completed", "legacy-failed"}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("management fence upgrade lost legacy rows: count=%d err=%v", count, err)
	}
	var legacyFailed RechargeRecord
	if err := db.First(&legacyFailed, "id = ?", "legacy-failed").Error; err != nil || legacyFailed.IdempotencyKey != "legacy:legacy-failed" || legacyFailed.Source != "legacy" {
		t.Fatalf("legacy row was not deterministically backfilled: row=%+v err=%v", legacyFailed, err)
	}
	budget := 20.0
	gateway := newBudgetGateway(&budget)
	server := gateway.server(t)
	t.Cleanup(server.Close)
	if _, err := executeRecharge(context.Background(), db, testClient(t, server), &legacyFailed); !errors.Is(err, ErrReconcileRequired) {
		t.Fatalf("legacy non-completed command must be inert: %v", err)
	}
	if gateway.userWrites != 0 {
		t.Fatalf("legacy command was re-executed: writes=%d", gateway.userWrites)
	}
}

func TestOperationsMigrationRollsBackAndCanRetry(t *testing.T) {
	db := openTestDB(t)
	if err := db.AutoMigrate(&migrationmodels.Migration{}, &models.CasbinRule{}); err != nil {
		t.Fatalf("base migration: %v", err)
	}
	if err := db.Exec(createRechargeTableDDL["sqlite"]).Error; err != nil {
		t.Fatalf("legacy table: %v", err)
	}
	now := time.Now().UTC()
	insert := `INSERT INTO litellmops_recharge (id,created_at,user_id,email,amount,before_budget,after_budget,raise_keys,keys_updated,operator,reason,idempotency_key,status) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`
	for _, id := range []string{"duplicate-a", "duplicate-b"} {
		if err := db.Exec(insert, id, now, "u-duplicate", "duplicate@example.com", 1, 10, 11, false, "[]", "old-admin", "legacy", "same-key", "completed").Error; err != nil {
			t.Fatalf("legacy duplicate row: %v", err)
		}
	}
	if err := migrateOperations(db, OperationsMigrationID.String()); err == nil {
		t.Fatal("conflicting legacy rows must fail closed")
	}
	if db.Migrator().HasColumn(&RechargeRecord{}, "updated_at") {
		t.Fatal("failed SQLite migration left a partially upgraded schema")
	}
	var count int64
	if err := db.Model(&migrationmodels.Migration{}).Where("version = ?", OperationsMigrationID.String()).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed migration must not write ledger: count=%d err=%v", count, err)
	}
	if err := db.Exec(`UPDATE litellmops_recharge SET idempotency_key = ? WHERE id = ?`, "different-key", "duplicate-b").Error; err != nil {
		t.Fatalf("resolve legacy conflict: %v", err)
	}
	if err := migrateOperations(db, OperationsMigrationID.String()); err != nil {
		t.Fatalf("retry after conflict resolution: %v", err)
	}
	if !db.Migrator().HasColumn(&RechargeRecord{}, "updated_at") {
		t.Fatal("successful retry did not upgrade schema")
	}
}

func TestGatewayModelResponseDoesNotExposeParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": []any{map[string]any{
			"model_name": "safe-model", "model_info": map[string]any{"id": "m-1", "mode": "chat"},
			"litellm_params": map[string]any{"litellm_provider": "provider-a", "base_model": "base-a", "api_key": "must-not-leak"},
		}}})
	}))
	t.Cleanup(server.Close)
	client := testClient(t, server)
	page, err := client.gatewayModels(context.Background())
	if err != nil || page.Total != 1 {
		t.Fatalf("models: %+v err=%v", page, err)
	}
	if page.Items[0].ID != "m-1" || page.Items[0].Name != "safe-model" || page.Items[0].Provider != "" || page.Items[0].BaseModel != "" || page.Items[0].Blocked != nil || page.Items[0].Manageable {
		t.Fatalf("unexpected canonical safe model projection: %+v", page.Items[0])
	}
	raw, _ := json.Marshal(page)
	if strings.Contains(string(raw), "must-not-leak") || strings.Contains(string(raw), "api_key") {
		t.Fatalf("unsafe gateway response: %s", raw)
	}
}

func TestUncertainManagementBudgetQuarantinesUserUntilReadback(t *testing.T) {
	withImmediateConfirmation(t)
	db := openTestDB(t)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	gateway.userWriteStatusOnce = http.StatusBadGateway
	gateway.applyBeforeFailure = true
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)
	target := int64(20_000_000)

	_, err := runFencedManagement(context.Background(), db, client, "operator-a", snapshot.UserID, "user", snapshot.UserID, "user_update",
		userMutationRequest{MaxBudgetUSDMicro: &target}, managementExpectedState{Kind: "user", Email: snapshot.Email, MaxBudgetUSDMicro: &target},
		func() (any, error) {
			return client.updateUser(context.Background(), snapshot.UserID, userMutationRequest{MaxBudgetUSDMicro: &target})
		})
	var commandErr *managementCommandResultError
	if !errors.As(err, &commandErr) || commandErr.Command.Status != ManagementResultUnverified {
		t.Fatalf("uncertain management write must persist quarantine: command=%+v err=%v", commandErr, err)
	}
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	writeAPIError(ginContext, err)
	if recorder.Code != http.StatusAccepted || !strings.Contains(recorder.Body.String(), `"max_budget_usd_micro":20000000`) || strings.Contains(recorder.Body.String(), "payload_hash") || strings.Contains(recorder.Body.String(), "before_state") || strings.Contains(recorder.Body.String(), "expected_state") {
		t.Fatalf("unsafe or invalid management 202 envelope: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if _, err := ApplyRecharge(context.Background(), db, client, snapshot, RechargeRequest{AmountUSDMicro: 5_000_000, IdempotencyKey: "blocked-by-management"}, "operator-b"); !errors.Is(err, ErrManagementPending) {
		t.Fatalf("recharge must be blocked by management quarantine: %v", err)
	}
	command := commandErr.Command
	if err := reconcileManagementCommand(context.Background(), db, client, &command, "operator-c"); err != nil {
		t.Fatalf("authoritative management readback should resolve applied: %v", err)
	}
	if command.Status != ManagementResolvedApplied {
		t.Fatalf("unexpected resolved status: %+v", command)
	}
	recharge, err := ApplyRecharge(context.Background(), db, client, snapshot, RechargeRequest{AmountUSDMicro: 5_000_000, IdempotencyKey: "blocked-by-management"}, "operator-b")
	if err != nil || recharge.Status != RechargeCompleted || recharge.TargetAfterUSDMicro == nil || *recharge.TargetAfterUSDMicro != 25_000_000 {
		t.Fatalf("recharge after management reconciliation lost budget: record=%+v err=%v", recharge, err)
	}
}

func TestUserMutationHandlerReturnsSafeAcceptedCommand(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	gateway.userWriteStatusOnce = http.StatusBadGateway
	gateway.applyBeforeFailure = true
	server := gateway.server(t)
	t.Cleanup(server.Close)
	t.Setenv(EnvLiteLLMBaseURL, server.URL)
	t.Setenv(EnvLiteLLMMasterKey, "test-master-key")
	handler := &requestHandler{runtime: business.Runtime{
		RequestDatabase: func(context.Context) (*gorm.DB, bool) { return db, true },
		Principal:       func(*gin.Context) security.Verifier { return &fakeVerifier{role: "admin"} },
	}}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PATCH("/users/:id", handler.updateUser)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/users/"+snapshot.ID, strings.NewReader(`{"max_budget_usd_micro":20000000}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted || !strings.Contains(recorder.Body.String(), `"command"`) ||
		!strings.Contains(recorder.Body.String(), `"requires_manual_review":false`) || strings.Contains(recorder.Body.String(), "payload_hash") || strings.Contains(recorder.Body.String(), "expected_state") {
		t.Fatalf("unexpected management mutation response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestManagementNoopRequiresStablePrewriteObservation(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	gateway.userWriteStatusOnce = http.StatusBadGateway
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)
	target := int64(10_000_000)

	_, err := runFencedManagement(context.Background(), db, client, "operator-a", snapshot.UserID, "user", snapshot.UserID, "user_update",
		userMutationRequest{MaxBudgetUSDMicro: &target}, managementExpectedState{Kind: "user", Email: snapshot.Email, MaxBudgetUSDMicro: &target},
		func() (any, error) {
			return client.updateUser(context.Background(), snapshot.UserID, userMutationRequest{MaxBudgetUSDMicro: &target})
		})
	var commandErr *managementCommandResultError
	if !errors.As(err, &commandErr) {
		t.Fatalf("expected durable uncertain command: %v", err)
	}
	command := commandErr.Command
	if err := reconcileManagementCommand(context.Background(), db, client, &command, "operator-b"); !errors.Is(err, ErrManagementUnverified) {
		t.Fatalf("same immediate value must not resolve: %v", err)
	}
	if command.Status != ManagementResultUnverified {
		t.Fatalf("same immediate value released quarantine: %+v", command)
	}
	oldCreated, oldObserved := time.Now().UTC().Add(-3*time.Minute), time.Now().UTC().Add(-6*time.Second)
	if err := db.Model(&ManagementCommand{}).Where("id = ?", command.ID).Updates(map[string]any{"created_at": oldCreated, "last_observed_at": oldObserved}).Error; err != nil {
		t.Fatalf("age command fixture: %v", err)
	}
	if err := db.First(&command, "id = ?", command.ID).Error; err != nil {
		t.Fatalf("reload command: %v", err)
	}
	if err := reconcileManagementCommand(context.Background(), db, client, &command, "operator-b"); err != nil || command.Status != ManagementResolvedNoop {
		t.Fatalf("stable pre-write value should resolve not applied: command=%+v err=%v", command, err)
	}
}

func TestUncertainKeyResetNeverAutoResolves(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db)
	seedRechargeUser(t, db)
	budget, keyBudget := 10.0, 10.0
	gateway := newBudgetGateway(&budget)
	token := "cccccccccccccccccccccccccccccccc"
	gateway.keys[token] = &keyBudget
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)
	zero := int64(0)

	_, err := runFencedManagement(context.Background(), db, client, "operator-a", "u-1", "key", "key-snapshot", "key_reset_spend",
		resetSpendRequest{}, managementExpectedState{Kind: "key", KeyPrefix: token[:keyHashPrefixLength], SpendUSDMicro: &zero, RequiresManualOnly: true},
		func() (any, error) {
			return nil, &UpstreamError{Path: "/key/{key}/reset_spend", StatusCode: http.StatusBadGateway, Uncertain: true}
		})
	var commandErr *managementCommandResultError
	if !errors.As(err, &commandErr) {
		t.Fatalf("expected reset quarantine: %v", err)
	}
	command := commandErr.Command
	if err := reconcileManagementCommand(context.Background(), db, client, &command, "operator-b"); !errors.Is(err, ErrManagementUnverified) {
		t.Fatalf("reset readback must require manual review: %v", err)
	}
	if command.Status != ManagementResultUnverified || !command.RequiresManualReview {
		t.Fatalf("reset command auto-resolved unexpectedly: %+v", command)
	}
}

func TestManualManagementResolveRequiresAgedStableAuthoritativeObservation(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db)
	seedRechargeUser(t, db)
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)
	expected := managementExpectedState{Kind: "user", RequiresManualOnly: true}
	probe := &ManagementCommand{UserID: "u-1", TargetID: "u-1"}
	_, digest, err := readManagementExpected(context.Background(), client, probe, expected)
	if err != nil {
		t.Fatalf("read authoritative fixture: %v", err)
	}
	now := time.Now().UTC()
	command := ManagementCommand{
		ID: newSnapshotID(), CreatedAt: now.Add(-3 * time.Minute), UpdatedAt: now, UserID: "u-1", UserEmail: "buyer@example.com",
		TargetType: "user", TargetID: "u-1", Action: "user_update", PayloadHash: "safe-digest",
		ExpectedState: managementExpectedJSON(expected), BeforeStateDigest: digest, Status: ManagementResultUnverified,
		RequiresManualReview: true, LastObservedDigest: digest, LastObservedAt: ptrTime(now.Add(-6 * time.Second)), Operator: "operator-a", Version: 1,
	}
	if err := db.Create(&command).Error; err != nil {
		t.Fatalf("create manual command: %v", err)
	}
	t.Setenv(EnvLiteLLMBaseURL, server.URL)
	t.Setenv(EnvLiteLLMMasterKey, "test-master-key")
	handler := &requestHandler{runtime: business.Runtime{
		RequestDatabase: func(context.Context) (*gorm.DB, bool) { return db, true },
		Principal:       func(*gin.Context) security.Verifier { return &fakeVerifier{role: "admin"} },
	}}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/management/commands/:id/resolve", handler.resolveManagementCommand)
	recorder := httptest.NewRecorder()
	body := strings.NewReader(`{"resolution":"not_applied","reason":"verified in LiteLLM","confirm_authoritative_state":true}`)
	request := httptest.NewRequest(http.MethodPost, "/management/commands/"+command.ID+"/resolve", body)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), ManagementResolvedNoop) {
		t.Fatalf("manual resolve failed: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var attempts int64
	if err := db.Model(&OperationAttempt{}).Where("operation_type = ? AND operation_id = ? AND step = ? AND operator = ?", "management", command.ID, "management_resolve", "test-user").Count(&attempts).Error; err != nil || attempts != 1 {
		t.Fatalf("manual resolve audit missing: count=%d err=%v", attempts, err)
	}
}

func TestCreateUserExplicitlyDisablesImplicitKey(t *testing.T) {
	var autoCreate any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(request.Body).Decode(&body)
		autoCreate = body["auto_create_key"]
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"user_id": "new-user", "user_email": "new@example.com"})
	}))
	t.Cleanup(server.Close)
	if _, err := testClient(t, server).createUser(context.Background(), userMutationRequest{Email: "new@example.com"}, false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if value, ok := autoCreate.(bool); !ok || value {
		t.Fatalf("user creation did not explicitly disable implicit key: %#v", autoCreate)
	}
}

func TestCreateByExistingEmailUsesRealUserRechargeFence(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	if _, _, err := reserveRecharge(context.Background(), db, snapshot,
		RechargeRequest{AmountUSDMicro: 1_000_000, IdempotencyKey: "existing-user-pending"}, "operator-a"); err != nil {
		t.Fatalf("reserve pending recharge: %v", err)
	}
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	server := gateway.server(t)
	t.Cleanup(server.Close)
	t.Setenv(EnvLiteLLMBaseURL, server.URL)
	t.Setenv(EnvLiteLLMMasterKey, "test-master-key")
	handler := &requestHandler{runtime: business.Runtime{
		RequestDatabase: func(context.Context) (*gorm.DB, bool) { return db, true },
		Principal:       func(*gin.Context) security.Verifier { return &fakeVerifier{role: "admin"} },
	}}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/users", handler.createUser)
	recorder := httptest.NewRecorder()
	body := strings.NewReader(`{"email":"buyer@example.com"}`)
	request := httptest.NewRequest(http.MethodPost, "/users", body)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "pending recharge") {
		t.Fatalf("existing email did not share real user fence: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestDirectRechargeReconcileHandlerCompletesAuthoritativeTarget(t *testing.T) {
	withImmediateConfirmation(t)
	db := openTestDB(t)
	migrateTestDB(t, db)
	snapshot := seedRechargeUser(t, db)
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	gateway.ignoreUserWrite = true
	server := gateway.server(t)
	t.Cleanup(server.Close)
	client := testClient(t, server)
	record, err := ApplyRecharge(context.Background(), db, client, snapshot, RechargeRequest{AmountUSDMicro: 5_000_000, IdempotencyKey: "direct-reconcile"}, "operator-a")
	if !errors.Is(err, ErrReconcileRequired) || record.Status != RechargeAppliedUnverified {
		t.Fatalf("create uncertain recharge: record=%+v err=%v", record, err)
	}
	t.Setenv(EnvLiteLLMBaseURL, server.URL)
	t.Setenv(EnvLiteLLMMasterKey, "test-master-key")
	handler := &requestHandler{runtime: business.Runtime{
		RequestDatabase: func(context.Context) (*gorm.DB, bool) { return db, true },
		Principal:       func(*gin.Context) security.Verifier { return &fakeVerifier{role: "admin"} },
	}}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/recharges/:id/reconcile", handler.reconcileRecharge)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/recharges/"+record.ID+"/reconcile", nil))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("still-unverified reconcile must return 202: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	gateway.mu.Lock()
	visible := 15.0
	gateway.budget = &visible
	gateway.mu.Unlock()
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/recharges/"+record.ID+"/reconcile", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("reconcile handler status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response RechargeRecord
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Status != RechargeCompleted {
		t.Fatalf("unexpected reconcile response: %+v err=%v", response, err)
	}
}

func TestGatewayHealthCanonicalContractAndWritesDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"status": "healthy", "db": "connected"})
	}))
	t.Cleanup(server.Close)
	health, err := testClient(t, server).gatewayHealth(context.Background())
	if err != nil || !health.Ready || health.Status != "ready" || health.DBStatus != "connected" || health.CheckedAt.IsZero() {
		t.Fatalf("unexpected canonical health: %+v err=%v", health, err)
	}
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	(&requestHandler{}).blockModel(ctx)
	if recorder.Code != http.StatusNotImplemented || !strings.Contains(recorder.Body.String(), "operation_disabled") {
		t.Fatalf("model write must fail closed: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestTrustedAutoApplyReturnsAcceptedPersistedOrder(t *testing.T) {
	withImmediateConfirmation(t)
	db := openTestDB(t)
	migrateTestDB(t, db)
	seedRechargeUser(t, db)
	now := time.Now().UTC()
	product := SalesProduct{
		ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now, Channel: "xianyu", Shop: "shop-a",
		ExternalItemID: "item-auto", SKU: "", Title: "auto credit", PriceCNYFen: 1000,
		CreditUSDMicro: 5_000_000, Enabled: true, AutoApply: true, Version: 1,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create auto product: %v", err)
	}
	budget := 10.0
	gateway := newBudgetGateway(&budget)
	gateway.ignoreUserWrite = true
	server := gateway.server(t)
	t.Cleanup(server.Close)
	t.Setenv(EnvLiteLLMBaseURL, server.URL)
	t.Setenv(EnvLiteLLMMasterKey, "test-master-key")
	handler := &requestHandler{runtime: business.Runtime{
		RequestDatabase: func(context.Context) (*gorm.DB, bool) { return db, true },
		Principal:       func(*gin.Context) security.Verifier { return &fakeVerifier{role: "admin"} },
	}}
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/sales/orders/import", nil)
	handler.receiveOrder(ctx, orderCreateRequest{
		Channel: "xianyu", Shop: "shop-a", ExternalOrderID: "auto-unverified", AdjustmentType: "credit",
		ExternalItemID: "item-auto", UserEmail: "buyer@example.com", PaidCNYFen: 1000, PaymentStatus: "paid",
	}, true)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("persisted auto-apply uncertainty must return 202: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var order SalesOrder
	if err := json.Unmarshal(recorder.Body.Bytes(), &order); err != nil || order.Status != OrderAppliedUnverified || order.RechargeID == "" {
		t.Fatalf("unexpected persisted order response: %+v err=%v", order, err)
	}
}

func TestOrderPayloadConflictIsNeverMaskedByPersistedTransientState(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db)
	request := orderCreateRequest{
		Channel: "xianyu", Shop: "shop-a", ExternalOrderID: "payload-conflict", AdjustmentType: "credit",
		UserEmail: "buyer@example.com", PaidCNYFen: 1000, PaymentStatus: "paid",
	}
	order, _, err := createSalesOrder(context.Background(), db, request, "operator-a", false)
	if err != nil {
		t.Fatalf("create order fixture: %v", err)
	}
	if err := db.Model(&SalesOrder{}).Where("id = ?", order.ID).Updates(map[string]any{"status": OrderRetryableFailed, "last_error_code": "recharge_pending"}).Error; err != nil {
		t.Fatalf("mark transient fixture: %v", err)
	}
	handler := &requestHandler{runtime: business.Runtime{
		RequestDatabase: func(context.Context) (*gorm.DB, bool) { return db, true },
		Principal:       func(*gin.Context) security.Verifier { return &fakeVerifier{role: "admin"} },
	}}
	request.PaidCNYFen = 999
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/sales/orders", nil)
	handler.receiveOrder(ctx, request, false)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "sales order conflict") {
		t.Fatalf("payload conflict was masked: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestGeneratedKeyIsNeverPersistedInAudit(t *testing.T) {
	db := openTestDB(t)
	migrateTestDB(t, db)
	seedRechargeUser(t, db)
	const rawSecret = "sk-one-time-secret-value"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"key": rawSecret, "user_id": "u-1", "key_alias": "new-key"})
	}))
	t.Cleanup(server.Close)
	client := testClient(t, server)
	request := keyMutationRequest{UserID: "u-1"}
	result, err := runFencedManagement(context.Background(), db, client, "operator-a", "u-1", "key", "u-1:new", "key_issue", request,
		managementExpectedState{Kind: "key", Email: "buyer@example.com", RequiresManualOnly: true},
		func() (any, error) { return client.issueKey(context.Background(), request) })
	if err != nil {
		t.Fatalf("durable key issue: %v", err)
	}
	response := result.(*oneTimeKeyResponse)
	if response.RawKey != rawSecret {
		t.Fatalf("issue key response: %+v", response)
	}
	var attempts []OperationAttempt
	if err := db.Find(&attempts).Error; err != nil {
		t.Fatalf("read audit: %v", err)
	}
	var commands []ManagementCommand
	if err := db.Find(&commands).Error; err != nil {
		t.Fatalf("read commands: %v", err)
	}
	stored, _ := json.Marshal(struct {
		Attempts []OperationAttempt
		Commands []ManagementCommand
	}{attempts, commands})
	if strings.Contains(string(stored), rawSecret) {
		t.Fatalf("raw key persisted in audit: %s", stored)
	}
}
