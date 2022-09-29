package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// AccountMember is the definition of a member of an account.
type AccountMember struct {
	ID       string                   `json:"id"`
	Code     string                   `json:"code"`
	User     AccountMemberUserDetails `json:"user"`
	Status   string                   `json:"status"`
	Roles    []AccountRole            `json:"roles,omitempty"`
	Policies []Policy                 `json:"policies,omitempty"`
}

// AccountMemberUserDetails outlines all the personal information about
// a member.
type AccountMemberUserDetails struct {
	ID                             string `json:"id"`
	FirstName                      string `json:"first_name"`
	LastName                       string `json:"last_name"`
	Email                          string `json:"email"`
	TwoFactorAuthenticationEnabled bool   `json:"two_factor_authentication_enabled"`
}

// AccountMembersListResponse represents the response from the list
// account members endpoint.
type AccountMembersListResponse struct {
	Result []AccountMember `json:"result"`
	Response
	ResultInfo `json:"result_info"`
}

// AccountMemberDetailResponse is the API response, containing a single
// account member.
type AccountMemberDetailResponse struct {
	Success  bool          `json:"success"`
	Errors   []string      `json:"errors"`
	Messages []string      `json:"messages"`
	Result   AccountMember `json:"result"`
}

// AccountMemberInvitation represents the invitation for a new member to
// the account.
type AccountMemberInvitation struct {
	Email    string   `json:"email"`
	Roles    []string `json:"roles,omitempty"`
	Policies []Policy `json:"policies,omitempty"`
	Status   string   `json:"status,omitempty"`
}

// AccountMembers returns all members of an account.
//
// API reference: https://api.cloudflare.com/#accounts-list-accounts
func (api *API) AccountMembers(ctx context.Context, accountID string, pageOpts PaginationOptions) ([]AccountMember, ResultInfo, error) {
	if accountID == "" {
		return []AccountMember{}, ResultInfo{}, ErrMissingAccountID
	}

	uri := buildURI(fmt.Sprintf("/accounts/%s/members", accountID), pageOpts)

	res, err := api.makeRequestContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return []AccountMember{}, ResultInfo{}, err
	}

	var accountMemberListresponse AccountMembersListResponse
	err = json.Unmarshal(res, &accountMemberListresponse)
	if err != nil {
		return []AccountMember{}, ResultInfo{}, fmt.Errorf("%s: %w", errUnmarshalError, err)
	}

	return accountMemberListresponse.Result, accountMemberListresponse.ResultInfo, nil
}

// CreateAccountMemberWithStatus invites a new member to join an account, allowing setting the status.
//
// Refer to the API reference for valid statuses.
//
// API reference: https://api.cloudflare.com/#account-members-add-member
func (api *API) CreateAccountMemberWithStatus(ctx context.Context, accountID string, emailAddress string, roles []string, status string) (AccountMember, error) {
	invite := AccountMemberInvitation{
		Email:    emailAddress,
		Roles:    roles,
		Policies: nil,
		Status:   status,
	}
	return api.CreateAccountMemberInternal(ctx, accountID, invite)
}

// CreateAccountMember invites a new member to join an account with roles.
// The member will be placed into "pending" status and receive an email confirmation.
// NOTE: If you are currently enrolled in Domain Scoped Roles, your roles will be converted to policies
// upon member invitation. We recommend upgrading to CreateAccountMemberWithPolicies to use policies.
//
// API reference: https://api.cloudflare.com/#account-members-add-member
func (api *API) CreateAccountMember(ctx context.Context, accountID string, emailAddress string, roles []string) (AccountMember, error) {
	invite := AccountMemberInvitation{
		Email:    emailAddress,
		Roles:    roles,
		Policies: nil,
		Status:   "",
	}
	return api.CreateAccountMemberInternal(ctx, accountID, invite)
}

// CreateAccountMemberWithRoles is a terse wrapper around the CreateAccountMember method
// for clarity on what permissions you're granting an AccountMember.
//
// API reference: https://api.cloudflare.com/#account-members-add-member
func (api *API) CreateAccountMemberWithRoles(ctx context.Context, accountID string, emailAddress string, roles []string) (AccountMember, error) {
	return api.CreateAccountMember(ctx, accountID, emailAddress, roles)
}

// CreateAccountMemberWithPolicies invites a new member to join your account with policies.
// Policies are the replacement to legacy "roles", which enables the newest feature Domain Scoped Roles.
//
// API documentation will be coming shortly. Blog post: https://blog.cloudflare.com/domain-scoped-roles-ga/
func (api *API) CreateAccountMemberWithPolicies(ctx context.Context, accountID string, emailAddress string, policies []Policy) (AccountMember, error) {
	invite := AccountMemberInvitation{
		Email:    emailAddress,
		Roles:    nil,
		Policies: policies,
		Status:   "",
	}
	return api.CreateAccountMemberInternal(ctx, accountID, invite)
}

// CreateAccountMemberInternal allows you to provide a raw AccountMemberInvitation to be processed
// and contains the logic for other CreateAccountMember* methods
func (api *API) CreateAccountMemberInternal(ctx context.Context, accountID string, invite AccountMemberInvitation) (AccountMember, error) {
	// make sure we have account
	if accountID == "" {
		return AccountMember{}, ErrMissingAccountID
	}

	// make sure we have roles OR policies
	roles := []AccountRole{}
	for i := 0; i < len(invite.Roles); i++ {
		roles = append(roles, AccountRole{ID: invite.Roles[i]})
	}
	err := validateRolesAndPolicies(roles, invite.Policies)
	if err != nil {
		return AccountMember{}, err
	}

	uri := fmt.Sprintf("/accounts/%s/members", accountID)
	res, err := api.makeRequestContext(ctx, http.MethodPost, uri, invite)
	if err != nil {
		return AccountMember{}, err
	}

	var accountMemberListResponse AccountMemberDetailResponse
	err = json.Unmarshal(res, &accountMemberListResponse)
	if err != nil {
		return AccountMember{}, fmt.Errorf("%s: %w", errUnmarshalError, err)
	}

	return accountMemberListResponse.Result, nil
}

// DeleteAccountMember removes a member from an account.
//
// API reference: https://api.cloudflare.com/#account-members-remove-member
func (api *API) DeleteAccountMember(ctx context.Context, accountID string, userID string) error {
	if accountID == "" {
		return ErrMissingAccountID
	}

	uri := fmt.Sprintf("/accounts/%s/members/%s", accountID, userID)

	_, err := api.makeRequestContext(ctx, http.MethodDelete, uri, nil)
	if err != nil {
		return err
	}

	return nil
}

// UpdateAccountMember modifies an existing account member.
//
// API reference: https://api.cloudflare.com/#account-members-update-member
func (api *API) UpdateAccountMember(ctx context.Context, accountID string, userID string, member AccountMember) (AccountMember, error) {
	if accountID == "" {
		return AccountMember{}, ErrMissingAccountID
	}

	err := validateRolesAndPolicies(member.Roles, member.Policies)
	if err != nil {
		return AccountMember{}, err
	}

	uri := fmt.Sprintf("/accounts/%s/members/%s", accountID, userID)

	res, err := api.makeRequestContext(ctx, http.MethodPut, uri, member)
	if err != nil {
		return AccountMember{}, err
	}

	var accountMemberListResponse AccountMemberDetailResponse
	err = json.Unmarshal(res, &accountMemberListResponse)
	if err != nil {
		return AccountMember{}, fmt.Errorf("%s: %w", errUnmarshalError, err)
	}

	return accountMemberListResponse.Result, nil
}

// AccountMember returns details of a single account member.
//
// API reference: https://api.cloudflare.com/#account-members-member-details
func (api *API) AccountMember(ctx context.Context, accountID string, memberID string) (AccountMember, error) {
	if accountID == "" {
		return AccountMember{}, ErrMissingAccountID
	}

	uri := fmt.Sprintf(
		"/accounts/%s/members/%s",
		accountID,
		memberID,
	)

	res, err := api.makeRequestContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return AccountMember{}, err
	}

	var accountMemberResponse AccountMemberDetailResponse
	err = json.Unmarshal(res, &accountMemberResponse)
	if err != nil {
		return AccountMember{}, fmt.Errorf("%s: %w", errUnmarshalError, err)
	}

	return accountMemberResponse.Result, nil
}

// validateRolesAndPolicies ensures either roles or policies are provided in
// CreateAccountMember requests, but not both
func validateRolesAndPolicies(roles []AccountRole, policies []Policy) error {
	hasRoles := roles != nil && len(roles) > 0
	hasPolicies := policies != nil && len(policies) > 0
	hasRolesOrPolicies := hasRoles || hasPolicies
	hasRolesAndPolicies := hasRoles && hasPolicies
	hasCorrectPermissions := hasRolesOrPolicies && !hasRolesAndPolicies
	if !hasCorrectPermissions {
		return ErrMissingMemberRolesOrPolicies
	}
	return nil
}
