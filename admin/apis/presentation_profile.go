package apis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/presentation"
	"github.com/mss-boot-io/mss-boot-admin/admin/service"
	bootpkg "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
)

const (
	presentationRequestMaxBytes = int64(presentation.MaxDocumentBytes + 16*1024)
	presentationMaxPage         = 1_000_000
)

var (
	errPresentationIfNoneMatchRequired = errors.New("presentation draft creation requires If-None-Match: *")
	errPresentationIfMatchRequired     = errors.New("presentation mutation requires If-Match")
)

// newPresentationProfileController preserves the package-local P1 test and
// compatibility entrypoint with an explicit empty, disabled composition.
func newPresentationProfileController() *PresentationProfileAPI {
	registry := presentation.MustNewFrozenRegistry()
	policy := presentation.MustNewAdoptionPolicy(
		presentation.AdoptionDisabled, nil, false, registry,
	)
	profileService, err := service.NewPresentationProfileService(registry, policy)
	if err != nil {
		panic(err)
	}
	api, err := NewPresentationProfileController(profileService)
	if err != nil {
		panic(err)
	}
	return api
}

// NewPresentationProfileController makes the presentation dependency explicit
// at the route composition boundary.
func NewPresentationProfileController(
	profileService *service.PresentationProfileService,
) (*PresentationProfileAPI, error) {
	if profileService == nil {
		return nil, errors.New("presentation profile service is required")
	}
	return &PresentationProfileAPI{Simple: controller.NewSimple(), service: profileService}, nil
}

type PresentationProfileAPI struct {
	*controller.Simple
	service *service.PresentationProfileService
}

func (*PresentationProfileAPI) GetAction(string) response.Action { return nil }

func (api *PresentationProfileAPI) profileService() *service.PresentationProfileService {
	if api != nil && api.service != nil {
		return api.service
	}
	return newPresentationProfileController().service
}

func (api *PresentationProfileAPI) Other(router *gin.RouterGroup) {
	router.GET("/presentation-capabilities", response.AuthHandler, api.Capabilities)
	router.POST("/presentation-profiles/validate", response.AuthHandler, api.Validate)
	router.GET("/presentation-profiles", response.AuthHandler, api.List)
	router.POST("/presentation-profiles", response.AuthHandler, api.CreateDraft)
	router.GET("/presentation-profiles/:id", response.AuthHandler, api.Get)
	router.PUT("/presentation-profiles/:id/draft", response.AuthHandler, api.ReplaceDraft)
	router.POST("/presentation-profiles/:id/publish", response.AuthHandler, api.Publish)
	router.GET("/presentation-profiles/:id/revisions", response.AuthHandler, api.ListRevisions)
	router.GET("/presentation-profiles/:id/revisions/:revision", response.AuthHandler, api.GetRevision)
	router.POST("/presentation-profiles/:id/rollback", response.AuthHandler, api.Rollback)
	router.GET("/presentation/effective/:pageKey", response.AuthHandler, api.Effective)
}

func (api *PresentationProfileAPI) Capabilities(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	response.Make(ctx).OK(dto.PresentationCapabilityListResponse{
		Items:        api.profileService().Capabilities(),
		RecoveryMode: api.profileService().RecoveryEnabled(),
		AdoptionMode: api.profileService().AdoptionMode(),
		ActivePages:  api.profileService().ActivePages(),
	})
}

func (api *PresentationProfileAPI) Validate(ctx *gin.Context) {
	request := &dto.PresentationValidationRequest{}
	if !bindPresentationJSON(ctx, request) {
		return
	}
	result := api.profileService().Validate(request.Document)
	outcome := "success"
	if !result.StructurallyValid || !result.SemanticallyValid {
		outcome = "validation_failed"
	}
	middleware.SetPresentationAuditMetadata(ctx, middleware.PresentationAuditMetadata{
		Transition: "validate", Outcome: outcome,
	})
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, result)
}

func (api *PresentationProfileAPI) List(ctx *gin.Context) {
	request, ok := presentationListRequest(ctx)
	if !ok {
		return
	}
	result, err := api.profileService().List(ctx, request)
	if err != nil {
		writePresentationServiceError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	response.Make(ctx).OK(result)
}

func (api *PresentationProfileAPI) Get(ctx *gin.Context) {
	profileID, ok := presentationPathID(ctx)
	if !ok {
		return
	}
	result, err := api.profileService().Get(ctx, profileID)
	if err != nil {
		writePresentationServiceError(ctx, err)
		return
	}
	setPresentationETag(ctx, result)
	response.Make(ctx).OK(result)
}

func (api *PresentationProfileAPI) CreateDraft(ctx *gin.Context) {
	if err := parsePresentationIfNoneMatch(ctx); err != nil {
		writePresentationPreconditionError(ctx, err)
		return
	}
	request := &dto.PresentationProfileCreateRequest{}
	if !bindPresentationJSON(ctx, request) {
		return
	}
	actorID, ok := presentationActorID(ctx)
	if !ok {
		return
	}
	setPresentationIdentityAudit(ctx, "create-draft", "started", "", request.PresentationProfileIdentity)
	result, err := api.profileService().CreateDraft(
		ctx,
		request.PresentationProfileIdentity,
		request.Document,
		actorID,
	)
	if err != nil {
		setPresentationIdentityAudit(ctx, "create-draft", presentationErrorOutcome(err), "", request.PresentationProfileIdentity)
		writePresentationServiceError(ctx, err)
		return
	}
	setPresentationResourceAudit(ctx, "create-draft", "success", result)
	setPresentationETag(ctx, result)
	ctx.JSON(http.StatusCreated, result)
}

func (api *PresentationProfileAPI) ReplaceDraft(ctx *gin.Context) {
	profileID, expectedVersion, ok := presentationMutationPrecondition(ctx)
	if !ok {
		return
	}
	request := &dto.PresentationDraftReplaceRequest{}
	if !bindPresentationJSON(ctx, request) {
		return
	}
	actorID, ok := presentationActorID(ctx)
	if !ok {
		return
	}
	middleware.SetPresentationAuditMetadata(ctx, middleware.PresentationAuditMetadata{
		AggregateID: profileID, Transition: "replace-draft", Outcome: "started",
	})
	result, err := api.profileService().ReplaceDraft(ctx, profileID, expectedVersion, request.Document, actorID)
	if writePresentationConflict(ctx, err, "replace-draft") {
		return
	}
	if err != nil {
		setPresentationErrorAudit(ctx, profileID, "replace-draft", err)
		writePresentationServiceError(ctx, err)
		return
	}
	setPresentationResourceAudit(ctx, "replace-draft", "success", result)
	setPresentationETag(ctx, result)
	response.Make(ctx).OK(result)
}

func (api *PresentationProfileAPI) Publish(ctx *gin.Context) {
	profileID, expectedVersion, ok := presentationMutationPrecondition(ctx)
	if !ok {
		return
	}
	actorID, ok := presentationActorID(ctx)
	if !ok {
		return
	}
	middleware.SetPresentationAuditMetadata(ctx, middleware.PresentationAuditMetadata{
		AggregateID: profileID, Transition: "publish", Outcome: "started",
	})
	result, err := api.profileService().Publish(
		ctx, profileID, expectedVersion, ctx.GetHeader("Idempotency-Key"), actorID,
	)
	if writePresentationConflict(ctx, err, "publish") {
		return
	}
	if err != nil {
		setPresentationErrorAudit(ctx, profileID, "publish", err)
		writePresentationServiceError(ctx, err)
		return
	}
	setPresentationResourceAudit(ctx, "publish", "success", result.Profile)
	setPresentationETag(ctx, result.Profile)
	ctx.JSON(http.StatusOK, result)
}

func (api *PresentationProfileAPI) Rollback(ctx *gin.Context) {
	profileID, expectedVersion, ok := presentationMutationPrecondition(ctx)
	if !ok {
		return
	}
	request := &dto.PresentationRollbackRequest{}
	if !bindPresentationJSON(ctx, request) {
		return
	}
	if request.Revision <= 0 {
		presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_REVISION_INVALID", "revision must be positive")
		return
	}
	actorID, ok := presentationActorID(ctx)
	if !ok {
		return
	}
	middleware.SetPresentationAuditMetadata(ctx, middleware.PresentationAuditMetadata{
		AggregateID: profileID, Transition: "rollback", Outcome: "started",
	})
	result, err := api.profileService().Rollback(
		ctx, profileID, expectedVersion, request.Revision, ctx.GetHeader("Idempotency-Key"), actorID,
	)
	if writePresentationConflict(ctx, err, "rollback") {
		return
	}
	if err != nil {
		setPresentationErrorAudit(ctx, profileID, "rollback", err)
		writePresentationServiceError(ctx, err)
		return
	}
	setPresentationResourceAudit(ctx, "rollback", "success", result.Profile)
	setPresentationETag(ctx, result.Profile)
	ctx.JSON(http.StatusOK, result)
}

func (api *PresentationProfileAPI) ListRevisions(ctx *gin.Context) {
	profileID, ok := presentationPathID(ctx)
	if !ok {
		return
	}
	page, pageSize, ok := presentationPagination(ctx)
	if !ok {
		return
	}
	result, err := api.profileService().ListRevisions(ctx, profileID, page, pageSize)
	if err != nil {
		writePresentationServiceError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	response.Make(ctx).OK(result)
}

func (api *PresentationProfileAPI) GetRevision(ctx *gin.Context) {
	profileID, ok := presentationPathID(ctx)
	if !ok {
		return
	}
	revision, err := strconv.ParseInt(ctx.Param("revision"), 10, 64)
	if err != nil || revision <= 0 {
		presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_REVISION_INVALID", "revision must be positive")
		return
	}
	result, err := api.profileService().GetRevision(ctx, profileID, revision)
	if err != nil {
		writePresentationServiceError(ctx, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	response.Make(ctx).OK(result)
}

func (api *PresentationProfileAPI) Effective(ctx *gin.Context) {
	verify := middleware.GetVerify(ctx)
	if verify == nil || strings.TrimSpace(verify.GetUserID()) == "" {
		presentationAPIError(ctx, http.StatusUnauthorized, "PRESENTATION_PRINCIPAL_REQUIRED", "authenticated principal is required")
		return
	}
	pageKey := strings.TrimSpace(ctx.Param("pageKey"))
	if pageKey == "" || len(pageKey) > 120 {
		presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_PAGE_INVALID", "page key is invalid")
		return
	}
	result, err := api.profileService().Effective(
		ctx,
		pageKey,
		strings.TrimSpace(verify.GetUserID()),
		strings.TrimSpace(verify.GetRoleID()),
	)
	if result != nil {
		ctx.Header("Cache-Control", "no-store")
		if err != nil {
			slog.Error("presentation effective read fell back", "pageKey", pageKey, "error", err)
		}
		response.Make(ctx).OK(result)
		return
	}
	writePresentationServiceError(ctx, err)
}

func presentationMutationPrecondition(ctx *gin.Context) (string, int64, bool) {
	profileID, ok := presentationPathID(ctx)
	if !ok {
		return "", 0, false
	}
	expectedVersion, err := parsePresentationIfMatch(ctx, profileID)
	if err != nil {
		middleware.SetPresentationAuditMetadata(ctx, middleware.PresentationAuditMetadata{
			AggregateID: profileID, Transition: presentationTransitionForRequest(ctx), Outcome: "bad_precondition",
		})
		writePresentationPreconditionError(ctx, err)
		return "", 0, false
	}
	return profileID, expectedVersion, true
}

func presentationListRequest(ctx *gin.Context) (dto.PresentationProfileListRequest, bool) {
	page, pageSize, ok := presentationPagination(ctx)
	if !ok {
		return dto.PresentationProfileListRequest{}, false
	}
	request := dto.PresentationProfileListRequest{
		Page: page, PageSize: pageSize, PageKey: strings.TrimSpace(ctx.Query("pageKey")),
		Scope: presentation.ScopeKind(strings.TrimSpace(ctx.Query("scope"))),
	}
	if len(request.PageKey) > 120 {
		presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_QUERY_INVALID", "page key filter is invalid")
		return dto.PresentationProfileListRequest{}, false
	}
	if request.Scope != "" && request.Scope != presentation.ScopeApplication &&
		request.Scope != presentation.ScopeRole && request.Scope != presentation.ScopeUser {
		presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_QUERY_INVALID", "scope filter is invalid")
		return dto.PresentationProfileListRequest{}, false
	}
	return request, true
}

func presentationPagination(ctx *gin.Context) (int, int, bool) {
	page, pageSize := 1, 20
	var err error
	if value := strings.TrimSpace(ctx.Query("page")); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil {
			presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_QUERY_INVALID", "page is invalid")
			return 0, 0, false
		}
	}
	if value := strings.TrimSpace(ctx.Query("pageSize")); value != "" {
		pageSize, err = strconv.Atoi(value)
		if err != nil {
			presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_QUERY_INVALID", "page size is invalid")
			return 0, 0, false
		}
	}
	if page < 1 || page > presentationMaxPage || pageSize < 1 || pageSize > service.PresentationListMaxPageSize {
		presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_QUERY_INVALID", "pagination is outside the supported range")
		return 0, 0, false
	}
	return page, pageSize, true
}

func bindPresentationJSON(ctx *gin.Context, destination any) bool {
	if ctx.Request == nil || ctx.Request.Body == nil {
		presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_PAYLOAD_INVALID", "JSON request body is required")
		return false
	}
	if ctx.Request.ContentLength > presentationRequestMaxBytes {
		presentationAPIError(ctx, http.StatusRequestEntityTooLarge, "PRESENTATION_PAYLOAD_TOO_LARGE", "presentation request is too large")
		return false
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, presentationRequestMaxBytes)
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			presentationAPIError(ctx, http.StatusRequestEntityTooLarge, "PRESENTATION_PAYLOAD_TOO_LARGE", "presentation request is too large")
			return false
		}
		presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_PAYLOAD_INVALID", "presentation request is invalid")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_PAYLOAD_INVALID", "presentation request must contain one JSON value")
		return false
	}
	return true
}

func presentationPathID(ctx *gin.Context) (string, bool) {
	profileID := strings.TrimSpace(ctx.Param("id"))
	if profileID == "" || len(profileID) > 64 {
		presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_ID_INVALID", "presentation profile identifier is invalid")
		return "", false
	}
	return profileID, true
}

func presentationActorID(ctx *gin.Context) (string, bool) {
	verify := middleware.GetVerify(ctx)
	if verify == nil {
		presentationAPIError(ctx, http.StatusUnauthorized, "PRESENTATION_ACTOR_REQUIRED", "authenticated presentation actor is required")
		return "", false
	}
	actorID := strings.TrimSpace(verify.GetUserID())
	if actorID == "" || len(actorID) > 64 {
		presentationAPIError(ctx, http.StatusForbidden, "PRESENTATION_ACTOR_INVALID", "authenticated presentation actor is invalid")
		return "", false
	}
	return actorID, true
}

func presentationProfileETag(profileID string, version int64) string {
	return strconv.Quote("presentation-profile-" + profileID + "-" + strconv.FormatInt(version, 10))
}

func setPresentationETag(ctx *gin.Context, resource *dto.PresentationProfileResource) {
	if resource == nil {
		return
	}
	setPresentationVersionETag(ctx, resource.ID, resource.Version)
}

func setPresentationVersionETag(ctx *gin.Context, profileID string, version int64) {
	ctx.Header("ETag", presentationProfileETag(profileID, version))
	ctx.Header("Cache-Control", "no-store")
}

func parsePresentationIfNoneMatch(ctx *gin.Context) error {
	values := ctx.Request.Header.Values("If-None-Match")
	if len(values) == 0 {
		return errPresentationIfNoneMatchRequired
	}
	if len(values) != 1 || strings.TrimSpace(values[0]) != "*" || values[0] != "*" {
		return errors.New("expected the canonical If-None-Match: * precondition")
	}
	return nil
}

func parsePresentationIfMatch(ctx *gin.Context, profileID string) (int64, error) {
	values := ctx.Request.Header.Values("If-Match")
	if len(values) == 0 {
		return 0, errPresentationIfMatchRequired
	}
	if len(values) != 1 {
		return 0, errors.New("expected one strong presentation profile ETag")
	}
	raw := strings.TrimSpace(values[0])
	if raw == "" || strings.HasPrefix(raw, "W/") || strings.Contains(raw, ",") || raw == "*" {
		return 0, errors.New("expected one strong presentation profile ETag")
	}
	value, err := strconv.Unquote(raw)
	if err != nil {
		return 0, errors.New("expected a quoted strong presentation profile ETag")
	}
	prefix := "presentation-profile-" + profileID + "-"
	if !strings.HasPrefix(value, prefix) {
		return 0, errors.New("presentation profile ETag identity does not match this resource")
	}
	versionText := strings.TrimPrefix(value, prefix)
	if versionText == "" || strings.Trim(versionText, "0123456789") != "" {
		return 0, errors.New("presentation profile ETag version is invalid")
	}
	version, err := strconv.ParseInt(versionText, 10, 64)
	if err != nil || version <= 0 || raw != presentationProfileETag(profileID, version) {
		return 0, errors.New("presentation profile ETag is not canonical")
	}
	return version, nil
}

func writePresentationPreconditionError(ctx *gin.Context, err error) {
	status, code := http.StatusBadRequest, "PRESENTATION_PRECONDITION_INVALID"
	if errors.Is(err, errPresentationIfMatchRequired) || errors.Is(err, errPresentationIfNoneMatchRequired) {
		status, code = http.StatusPreconditionRequired, "PRESENTATION_PRECONDITION_REQUIRED"
	}
	presentationAPIError(ctx, status, code, err.Error())
}

func writePresentationConflict(ctx *gin.Context, err error, transition string) bool {
	var conflict *service.PresentationRevisionConflictError
	if !errors.As(err, &conflict) {
		return false
	}
	setPresentationConflictAudit(ctx, transition, conflict.Current)
	setPresentationVersionETag(ctx, conflict.Current.ID, conflict.Current.Version)
	ctx.AbortWithStatusJSON(http.StatusPreconditionFailed, dto.PresentationConflictResponse{
		Success:      false,
		Status:       "error",
		Code:         http.StatusPreconditionFailed,
		ErrorCode:    "PRESENTATION_REVISION_CONFLICT",
		ErrorMessage: "presentation profile changed since it was loaded",
		TraceID:      bootpkg.GenerateMsgIDFromContext(ctx),
		Data:         dto.PresentationConflictResponseData{Current: conflict.Current},
	})
	return true
}

func writePresentationServiceError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrPresentationNotFound), errors.Is(err, service.ErrPresentationRevisionNotFound):
		presentationAPIError(ctx, http.StatusNotFound, "PRESENTATION_NOT_FOUND", "presentation resource was not found")
	case errors.Is(err, service.ErrPresentationIdentityConflict):
		presentationAPIError(ctx, http.StatusConflict, "PRESENTATION_IDENTITY_CONFLICT", "presentation profile identity already exists")
	case errors.Is(err, service.ErrPresentationIdempotencyConflict):
		presentationAPIError(ctx, http.StatusConflict, "PRESENTATION_IDEMPOTENCY_CONFLICT", "idempotency key was reused for another request")
	case errors.Is(err, service.ErrPresentationInvalidTransition):
		presentationAPIError(ctx, http.StatusConflict, "PRESENTATION_TRANSITION_INVALID", "presentation transition is not allowed from the current state")
	case errors.Is(err, service.ErrPresentationIdempotencyKey):
		presentationAPIError(ctx, http.StatusBadRequest, "PRESENTATION_IDEMPOTENCY_KEY_INVALID", "Idempotency-Key is required and must be 8 to 200 visible ASCII characters")
	case errors.Is(err, service.ErrPresentationInvalidDocument),
		errors.Is(err, service.ErrPresentationIdentityMismatch),
		errors.Is(err, service.ErrPresentationInvalidScope),
		errors.Is(err, service.ErrPresentationSubjectNotFound):
		presentationAPIError(ctx, http.StatusUnprocessableEntity, "PRESENTATION_VALIDATION_FAILED", "presentation document or scope is invalid")
	default:
		response.Make(ctx).AddError(err).Log.Error("presentation service unavailable")
		presentationAPIError(ctx, http.StatusServiceUnavailable, "PRESENTATION_UNAVAILABLE", "presentation service is unavailable")
	}
}

func presentationAPIError(ctx *gin.Context, status int, code, message string) {
	response.Make(ctx).AddError(response.NewError(code, message)).Err(status)
}

func presentationTransitionForRequest(ctx *gin.Context) string {
	transition, _, ok := presentationAuditRouteForAPI(ctx.Request.Method, ctx.Request.URL.Path)
	if ok {
		return transition
	}
	return "mutation"
}

func presentationAuditRouteForAPI(method, path string) (string, string, bool) {
	const prefix = "/admin/api/presentation-profiles"
	remainder := strings.TrimPrefix(path, prefix+"/")
	segments := strings.Split(remainder, "/")
	if len(segments) != 2 {
		return "", "", false
	}
	switch {
	case method == http.MethodPut && segments[1] == "draft":
		return "replace-draft", segments[0], true
	case method == http.MethodPost && segments[1] == "publish":
		return "publish", segments[0], true
	case method == http.MethodPost && segments[1] == "rollback":
		return "rollback", segments[0], true
	default:
		return "", "", false
	}
}

func setPresentationIdentityAudit(
	ctx *gin.Context,
	transition string,
	outcome string,
	aggregateID string,
	identity dto.PresentationProfileIdentity,
) {
	metadata := middleware.PresentationAuditMetadata{
		AggregateID: aggregateID, PageKey: identity.PageKey, Scope: string(identity.Scope),
		SubjectPresent: strings.TrimSpace(identity.SubjectID) != "", Transition: transition, Outcome: outcome,
	}
	if metadata.SubjectPresent {
		metadata.SubjectFingerprint = presentationSubjectFingerprint(identity.Scope, identity.SubjectID)
	}
	middleware.SetPresentationAuditMetadata(ctx, metadata)
}

func setPresentationResourceAudit(ctx *gin.Context, transition, outcome string, resource *dto.PresentationProfileResource) {
	if resource == nil {
		return
	}
	metadata := middleware.PresentationAuditMetadata{
		AggregateID: resource.ID, PageKey: resource.PageKey, Scope: string(resource.Scope),
		SubjectPresent: resource.SubjectID != "", Transition: transition, Outcome: outcome,
		AggregateVersion: resource.Version, PublishedRevision: resource.PublishedRevision,
	}
	if resource.SubjectID != "" {
		metadata.SubjectFingerprint = presentationSubjectFingerprint(resource.Scope, resource.SubjectID)
	}
	if resource.Draft != nil {
		metadata.DefinitionHash, metadata.ContentDigest = resource.Draft.DefinitionHash, resource.Draft.Digest
	} else if resource.Published != nil {
		metadata.DefinitionHash, metadata.ContentDigest = resource.Published.DefinitionHash, resource.Published.ContentDigest
	}
	middleware.SetPresentationAuditMetadata(ctx, metadata)
}

func setPresentationConflictAudit(
	ctx *gin.Context,
	transition string,
	resource *dto.PresentationConflictResource,
) {
	if resource == nil {
		return
	}
	middleware.SetPresentationAuditMetadata(ctx, middleware.PresentationAuditMetadata{
		AggregateID: resource.ID, Transition: transition, Outcome: "conflict",
		AggregateVersion: resource.Version,
	})
}

func setPresentationErrorAudit(ctx *gin.Context, aggregateID, transition string, err error) {
	middleware.SetPresentationAuditMetadata(ctx, middleware.PresentationAuditMetadata{
		AggregateID: aggregateID, Transition: transition, Outcome: presentationErrorOutcome(err),
	})
}

func presentationErrorOutcome(err error) string {
	switch {
	case errors.Is(err, service.ErrPresentationInvalidDocument),
		errors.Is(err, service.ErrPresentationIdentityMismatch),
		errors.Is(err, service.ErrPresentationInvalidScope),
		errors.Is(err, service.ErrPresentationSubjectNotFound):
		return "validation_failed"
	case errors.Is(err, service.ErrPresentationIdempotencyConflict):
		return "idempotency_conflict"
	case errors.Is(err, service.ErrPresentationInvalidTransition):
		return "invalid_transition"
	default:
		return "database_failure"
	}
}

func presentationSubjectFingerprint(scope presentation.ScopeKind, subject string) string {
	sum := sha256.Sum256([]byte("presentation-subject\x00" + string(scope) + "\x00" + subject))
	return "sha256:" + hex.EncodeToString(sum[:])
}
