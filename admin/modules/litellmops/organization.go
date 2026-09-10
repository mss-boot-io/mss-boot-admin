package litellmops

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RemoteOrg is a tolerated LiteLLM organization projection.
type RemoteOrg struct {
	OrganizationID string            `json:"organization_id"`
	Alias          string            `json:"organization_alias"`
	Spend          float64           `json:"spend"`
	MaxBudget      *float64          `json:"max_budget"`
	BudgetDuration *string           `json:"budget_duration"`
	TPMLimit       *int64            `json:"tpm_limit"`
	RPMLimit       *int64            `json:"rpm_limit"`
	Models         []string          `json:"models"`
	Members        []RemoteOrgMember `json:"members"`
	Teams          []RemoteTeam      `json:"teams"`
}

// RemoteOrgMember is one organization membership row.
type RemoteOrgMember struct {
	UserID                  string   `json:"user_id"`
	UserEmail               string   `json:"user_email"`
	Role                    string   `json:"role"`
	MaxBudgetInOrganization *float64 `json:"max_budget_in_organization"`
}

// RemoteTeam is a tolerated LiteLLM team projection.
type RemoteTeam struct {
	TeamID         string   `json:"team_id"`
	Alias          string   `json:"team_alias"`
	OrganizationID string   `json:"organization_id"`
	Spend          float64  `json:"spend"`
	MaxBudget      *float64 `json:"max_budget"`
	Models         []string `json:"models"`
}

func asObject(value any) map[string]any {
	object, ok := value.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return object
}

func asObjectSlice(value any) []map[string]any {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if object, ok := item.(map[string]any); ok {
			out = append(out, object)
		}
	}
	return out
}

func parseOrg(item map[string]any) RemoteOrg {
	budget := asObject(item["litellm_budget_table"])
	org := RemoteOrg{
		OrganizationID: mapString(item, "organization_id"),
		Alias:          mapString(item, "organization_alias"),
		Spend:          mapFloat(item, "spend"),
		MaxBudget:      firstFloatPtr(budget, item, "max_budget"),
		BudgetDuration: firstStringPtr(budget, item, "budget_duration"),
		TPMLimit:       firstInt64Ptr(budget, item, "tpm_limit"),
		RPMLimit:       firstInt64Ptr(budget, item, "rpm_limit"),
		Models:         mapStringSlice(item, "models"),
	}
	for _, member := range asObjectSlice(item["members"]) {
		org.Members = append(org.Members, RemoteOrgMember{
			UserID:                  mapString(member, "user_id"),
			UserEmail:               mapString(member, "user_email", "email"),
			Role:                    mapString(member, "user_role", "role"),
			MaxBudgetInOrganization: mapFloatPtr(member, "max_budget_in_organization"),
		})
	}
	for _, team := range asObjectSlice(item["teams"]) {
		org.Teams = append(org.Teams, parseTeam(team))
	}
	return org
}

func parseTeam(item map[string]any) RemoteTeam {
	budget := asObject(item["litellm_budget_table"])
	return RemoteTeam{
		TeamID:         mapString(item, "team_id"),
		Alias:          mapString(item, "team_alias"),
		OrganizationID: mapString(item, "organization_id"),
		Spend:          mapFloat(item, "spend"),
		MaxBudget:      firstFloatPtr(budget, item, "max_budget"),
		Models:         mapStringSlice(item, "models"),
	}
}

func firstFloatPtr(primary, fallback map[string]any, key string) *float64 {
	if value := mapFloatPtr(primary, key); value != nil {
		return value
	}
	return mapFloatPtr(fallback, key)
}

func firstStringPtr(primary, fallback map[string]any, key string) *string {
	if value := mapStringPtr(primary, key); value != nil {
		return value
	}
	return mapStringPtr(fallback, key)
}

func firstInt64Ptr(primary, fallback map[string]any, key string) *int64 {
	if value := mapInt64Ptr(primary, key); value != nil {
		return value
	}
	return mapInt64Ptr(fallback, key)
}

func decodeOrgList(raw json.RawMessage) ([]RemoteOrg, error) {
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		var envelope map[string]json.RawMessage
		if envErr := json.Unmarshal(raw, &envelope); envErr != nil {
			return nil, err
		}
		for _, key := range []string{"organizations", "data", "orgs"} {
			if wrapped, ok := envelope[key]; ok {
				if wrapErr := json.Unmarshal(wrapped, &items); wrapErr == nil {
					break
				}
			}
		}
	}
	orgs := make([]RemoteOrg, 0, len(items))
	for _, item := range items {
		org := parseOrg(item)
		if org.OrganizationID == "" {
			return nil, errors.New("litellmops organization is missing organization_id")
		}
		orgs = append(orgs, org)
	}
	return orgs, nil
}

func decodeOrg(raw json.RawMessage) (*RemoteOrg, error) {
	var item map[string]any
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	if nested, ok := item["organization"].(map[string]any); ok {
		item = nested
	}
	org := parseOrg(item)
	if org.OrganizationID == "" {
		return nil, errors.New("litellmops organization is missing organization_id")
	}
	return &org, nil
}

func decodeTeamList(raw json.RawMessage) ([]RemoteTeam, error) {
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		var envelope map[string]json.RawMessage
		if envErr := json.Unmarshal(raw, &envelope); envErr != nil {
			return nil, err
		}
		for _, key := range []string{"teams", "data"} {
			if wrapped, ok := envelope[key]; ok {
				if wrapErr := json.Unmarshal(wrapped, &items); wrapErr == nil {
					break
				}
			}
		}
	}
	teams := make([]RemoteTeam, 0, len(items))
	for _, item := range items {
		team := parseTeam(item)
		if team.TeamID == "" {
			continue
		}
		teams = append(teams, team)
	}
	return teams, nil
}

func decodeTeam(raw json.RawMessage) (*RemoteTeam, error) {
	var item map[string]any
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	if nested, ok := item["team_info"].(map[string]any); ok {
		item = nested
	}
	if nested, ok := item["team"].(map[string]any); ok {
		item = nested
	}
	team := parseTeam(item)
	if team.TeamID == "" {
		return nil, errors.New("litellmops team is missing team_id")
	}
	return &team, nil
}

func (client *Client) ListOrganizations(ctx context.Context) ([]RemoteOrg, error) {
	var raw json.RawMessage
	if err := client.getJSON(ctx, "/organization/list", nil, &raw); err != nil {
		return nil, err
	}
	return decodeOrgList(raw)
}

func (client *Client) GetOrganization(ctx context.Context, organizationID string) (*RemoteOrg, error) {
	var raw json.RawMessage
	if err := client.getJSON(ctx, "/organization/info", url.Values{"organization_id": {organizationID}}, &raw); err != nil {
		return nil, err
	}
	return decodeOrg(raw)
}

func (client *Client) CreateOrganization(ctx context.Context, alias string, maxBudget *float64, models []string) (*RemoteOrg, error) {
	payload := map[string]any{"organization_alias": strings.TrimSpace(alias)}
	if maxBudget != nil {
		payload["max_budget"] = *maxBudget
	}
	if len(models) > 0 {
		payload["models"] = models
	}
	var raw json.RawMessage
	if err := client.postJSON(ctx, "/organization/new", payload, &raw); err != nil {
		return nil, err
	}
	return decodeOrg(raw)
}

func (client *Client) UpdateOrganization(ctx context.Context, fields map[string]any) (*RemoteOrg, error) {
	var raw json.RawMessage
	if err := client.doJSON(ctx, http.MethodPatch, "/organization/update", nil, fields, &raw); err != nil {
		return nil, err
	}
	return decodeOrg(raw)
}

func (client *Client) DeleteOrganization(ctx context.Context, organizationID string) error {
	return client.doJSON(ctx, http.MethodDelete, "/organization/delete", nil, map[string]any{
		"organization_ids": []string{organizationID},
	}, nil)
}

func (client *Client) AddOrganizationMember(ctx context.Context, organizationID, userID, userEmail, role string) error {
	if role == "" {
		role = "internal_user"
	}
	userID = strings.TrimSpace(userID)
	userEmail = strings.TrimSpace(userEmail)
	if userID == "" && userEmail != "" {
		users, err := client.ListUsers(ctx)
		if err != nil {
			return err
		}
		for _, user := range users {
			if strings.EqualFold(user.Email, userEmail) {
				userID = user.UserID
				break
			}
		}
	}
	member := map[string]any{"role": role}
	if userID != "" {
		member["user_id"] = userID
	} else if userEmail != "" {
		member["user_email"] = userEmail
	} else {
		return errors.New("user_id or user_email is required")
	}
	return client.postJSON(ctx, "/organization/member_add", map[string]any{
		"organization_id": organizationID,
		"member":          member,
	}, nil)
}

func (client *Client) DeleteOrganizationMember(ctx context.Context, organizationID, userID, userEmail string) error {
	payload := map[string]any{"organization_id": organizationID}
	if userID != "" {
		payload["user_id"] = userID
	}
	if userEmail != "" {
		payload["user_email"] = userEmail
	}
	return client.doJSON(ctx, http.MethodDelete, "/organization/member_delete", nil, payload, nil)
}

func (client *Client) ListTeams(ctx context.Context) ([]RemoteTeam, error) {
	var raw json.RawMessage
	if err := client.getJSON(ctx, "/team/list", nil, &raw); err != nil {
		return nil, err
	}
	return decodeTeamList(raw)
}

func (client *Client) CreateTeam(ctx context.Context, alias, organizationID string, maxBudget *float64, models []string) (*RemoteTeam, error) {
	payload := map[string]any{
		"team_alias":      strings.TrimSpace(alias),
		"organization_id": organizationID,
	}
	if maxBudget != nil {
		payload["max_budget"] = *maxBudget
	}
	if len(models) > 0 {
		payload["models"] = models
	}
	var raw json.RawMessage
	if err := client.postJSON(ctx, "/team/new", payload, &raw); err != nil {
		return nil, err
	}
	return decodeTeam(raw)
}

func (client *Client) UpdateTeam(ctx context.Context, fields map[string]any) error {
	return client.postJSON(ctx, "/team/update", fields, nil)
}

func (client *Client) DeleteTeam(ctx context.Context, teamID string) error {
	return client.postJSON(ctx, "/team/delete", map[string]any{"team_ids": []string{teamID}}, nil)
}

func (handler *requestHandler) litellmClient(ctx *gin.Context) (*Client, bool) {
	client, err := NewClientFromEnv()
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return nil, false
	}
	return client, true
}

func (handler *requestHandler) listOrganizations(ctx *gin.Context) {
	client, ok := handler.litellmClient(ctx)
	if !ok {
		return
	}
	orgs, err := client.ListOrganizations(ctx.Request.Context())
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"items": orgs, "total": len(orgs)})
}

func (handler *requestHandler) getOrganization(ctx *gin.Context) {
	client, ok := handler.litellmClient(ctx)
	if !ok {
		return
	}
	org, err := client.GetOrganization(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, org)
}

type orgCreateRequest struct {
	Alias     string   `json:"organization_alias"`
	MaxBudget *float64 `json:"max_budget"`
	Models    []string `json:"models"`
}

func (handler *requestHandler) createOrganization(ctx *gin.Context) {
	client, ok := handler.litellmClient(ctx)
	if !ok {
		return
	}
	var request orgCreateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Alias) == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "organization_alias is required"})
		return
	}
	org, err := client.CreateOrganization(ctx.Request.Context(), request.Alias, request.MaxBudget, request.Models)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if err := handler.writeOrgAudit(ctx, "create_org", org.OrganizationID, request.Alias, map[string]any{"max_budget": request.MaxBudget}); err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, org)
}

type orgUpdateRequest struct {
	Alias     *string  `json:"organization_alias"`
	MaxBudget *float64 `json:"max_budget"`
	Models    []string `json:"models"`
}

func (handler *requestHandler) updateOrganization(ctx *gin.Context) {
	client, ok := handler.litellmClient(ctx)
	if !ok {
		return
	}
	var request orgUpdateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid organization update"})
		return
	}
	fields := map[string]any{"organization_id": ctx.Param("id")}
	if request.Alias != nil {
		fields["organization_alias"] = *request.Alias
	}
	if request.MaxBudget != nil {
		fields["max_budget"] = *request.MaxBudget
	}
	if request.Models != nil {
		fields["models"] = request.Models
	}
	org, err := client.UpdateOrganization(ctx.Request.Context(), fields)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if err := handler.writeOrgAudit(ctx, "update_org", ctx.Param("id"), org.Alias, fields); err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, org)
}

func (handler *requestHandler) deleteOrganization(ctx *gin.Context) {
	client, ok := handler.litellmClient(ctx)
	if !ok {
		return
	}
	id := ctx.Param("id")
	if err := client.DeleteOrganization(ctx.Request.Context(), id); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if err := handler.writeOrgAudit(ctx, "delete_org", id, "", nil); err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"deleted": id})
}

func (handler *requestHandler) rechargeOrganization(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{
		"error": "organization recharge is disabled until it uses the durable safe-recharge ledger",
		"code":  "operation_disabled",
	})
}

type orgMemberRequest struct {
	UserID    string `json:"user_id"`
	UserEmail string `json:"user_email"`
	Role      string `json:"role"`
}

func (handler *requestHandler) addOrganizationMember(ctx *gin.Context) {
	client, ok := handler.litellmClient(ctx)
	if !ok {
		return
	}
	var request orgMemberRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || (strings.TrimSpace(request.UserEmail) == "" && strings.TrimSpace(request.UserID) == "") {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "user_email or user_id is required"})
		return
	}
	id := ctx.Param("id")
	if err := client.AddOrganizationMember(ctx.Request.Context(), id, request.UserID, request.UserEmail, request.Role); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if err := handler.writeOrgAudit(ctx, "add_member", id, request.UserEmail, request); err != nil {
		writeAPIError(ctx, err)
		return
	}
	org, err := client.GetOrganization(ctx.Request.Context(), id)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"added": request.UserEmail})
		return
	}
	ctx.JSON(http.StatusOK, org)
}

func (handler *requestHandler) deleteOrganizationMember(ctx *gin.Context) {
	client, ok := handler.litellmClient(ctx)
	if !ok {
		return
	}
	var request orgMemberRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid member delete"})
		return
	}
	id := ctx.Param("id")
	if err := client.DeleteOrganizationMember(ctx.Request.Context(), id, request.UserID, request.UserEmail); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if err := handler.writeOrgAudit(ctx, "delete_member", id, request.UserEmail, request); err != nil {
		writeAPIError(ctx, err)
		return
	}
	org, err := client.GetOrganization(ctx.Request.Context(), id)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"deleted": request.UserEmail})
		return
	}
	ctx.JSON(http.StatusOK, org)
}

type teamCreateRequest struct {
	Alias     string   `json:"team_alias"`
	MaxBudget *float64 `json:"max_budget"`
	Models    []string `json:"models"`
}

func (handler *requestHandler) createTeam(ctx *gin.Context) {
	client, ok := handler.litellmClient(ctx)
	if !ok {
		return
	}
	var request teamCreateRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Alias) == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "team_alias is required"})
		return
	}
	orgID := ctx.Param("id")
	team, err := client.CreateTeam(ctx.Request.Context(), request.Alias, orgID, request.MaxBudget, request.Models)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if err := handler.writeOrgAudit(ctx, "create_team", orgID, request.Alias, request); err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, team)
}

func (handler *requestHandler) deleteTeam(ctx *gin.Context) {
	client, ok := handler.litellmClient(ctx)
	if !ok {
		return
	}
	teamID := ctx.Param("teamId")
	if err := client.DeleteTeam(ctx.Request.Context(), teamID); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if err := handler.writeOrgAudit(ctx, "delete_team", teamID, "", nil); err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"deleted": teamID})
}

func (handler *requestHandler) rechargeTeam(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{
		"error": "team recharge is disabled until it uses the durable safe-recharge ledger",
		"code":  "operation_disabled",
	})
}

func (handler *requestHandler) writeOrgAudit(ctx *gin.Context, action, targetID, targetName string, detail any) error {
	db, ok := handler.runtime.RequestDatabase(ctx.Request.Context())
	if !ok || db == nil {
		return errors.New("litellmops organization audit database unavailable")
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return errors.New("litellmops organization audit detail is invalid")
	}
	operator := ""
	if principal := handler.runtime.Principal(ctx); principal != nil {
		operator = strings.TrimSpace(principal.GetUsername())
		if operator == "" {
			operator = strings.TrimSpace(principal.GetUserID())
		}
	}
	return db.Create(&OrgAuditRecord{
		ID:         newSnapshotID(),
		CreatedAt:  time.Now().UTC(),
		Action:     action,
		TargetID:   targetID,
		TargetName: targetName,
		Operator:   operator,
		Detail:     string(encoded),
	}).Error
}

// OrgAuditRecord is an append-only log of organization/team mutations.
type OrgAuditRecord struct {
	ID         string    `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	Action     string    `gorm:"column:action;type:varchar(32);not null" json:"action"`
	TargetID   string    `gorm:"column:target_id;type:varchar(64);not null" json:"target_id"`
	TargetName string    `gorm:"column:target_name;type:varchar(128)" json:"target_name"`
	Operator   string    `gorm:"column:operator;type:varchar(128)" json:"operator"`
	Detail     string    `gorm:"column:detail;type:text" json:"detail"`
}

// TableName implements gorm.Tabler.
func (OrgAuditRecord) TableName() string { return "litellmops_org_audit" }
