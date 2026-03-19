/*
 * SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package authz

import (
	cdb "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	cdbm "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/model"
)

const (
	// ProviderAdminRole is the role that gives Provider Admin access to an org
	ProviderAdminRole = "FORGE_PROVIDER_ADMIN"
	// ProviderViewerRole is the role that gives Provider Viewer access to an org
	ProviderViewerRole = "FORGE_PROVIDER_VIEWER"
	// TenantAdminRole is the role that gives Tenant Admin access to an org
	TenantAdminRole = "FORGE_TENANT_ADMIN"
)

// ValidateOrgMembership validates if a given user is member of an org
func ValidateOrgMembership(user *cdbm.User, org string) (bool, error) {
	_, err := user.OrgData.GetOrgByName(org)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ValidateUserRoles validates user roles using the appropriate method based on user data
func ValidateUserRoles(user *cdbm.User, orgName string, teamName *string, targetRoles ...string) bool {
	userOrgDetails, err := user.OrgData.GetOrgByName(orgName)
	if err != nil {
		return false
	}
	return ValidateUserRolesInOrg(*userOrgDetails, teamName, targetRoles...)
}

// ValidateUserRolesInOrg checks if user has any of the specified roles (not all)
func ValidateUserRolesInOrg(userOrgDetails cdbm.Org, teamName *string, targetRoles ...string) bool {
	var userHasRole bool

	targetRoleMap := map[string]bool{}
	for _, targetRole := range targetRoles {
		targetRoleMap[targetRole] = true
	}

	if teamName == nil {
		// Check if user has an org level role
		for _, userOrgRole := range userOrgDetails.Roles {
			_, ok := targetRoleMap[userOrgRole]
			if ok {
				userHasRole = true
				break
			}
		}
	} else {
		// Check if user has a team role
		for _, userTeamDetails := range userOrgDetails.Teams {
			if userTeamDetails.Name != *teamName {
				continue
			}

			for _, userTeamRole := range userTeamDetails.Roles {
				_, ok := targetRoleMap[userTeamRole]
				if ok {
					userHasRole = true
					break
				}
			}
		}
	}

	return userHasRole
}
