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

package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	temporalEnums "go.temporal.io/api/enums/v1"
	tClient "go.temporal.io/sdk/client"
	tp "go.temporal.io/sdk/temporal"

	"github.com/nvidia/bare-metal-manager-rest/api/internal/config"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/handler/util/common"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/model"
	sc "github.com/nvidia/bare-metal-manager-rest/api/pkg/client/site"
	auth "github.com/nvidia/bare-metal-manager-rest/auth/pkg/authorization"
	cutil "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"
	rlav1 "github.com/nvidia/bare-metal-manager-rest/workflow-schema/rla/protobuf/v1"
	"github.com/nvidia/bare-metal-manager-rest/workflow/pkg/queue"
)

// ~~~~~ Get Task Handler ~~~~~ //

// GetTaskHandler is the API Handler for getting a Task by ID
type GetTaskHandler struct {
	dbSession  *cdb.Session
	tc         tClient.Client
	scp        *sc.ClientPool
	cfg        *config.Config
	tracerSpan *cutil.TracerSpan
}

// NewGetTaskHandler initializes and returns a new handler for getting a Task
func NewGetTaskHandler(dbSession *cdb.Session, tc tClient.Client, scp *sc.ClientPool, cfg *config.Config) GetTaskHandler {
	return GetTaskHandler{
		dbSession:  dbSession,
		tc:         tc,
		scp:        scp,
		cfg:        cfg,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Get a Task
// @Description Get a Task by UUID
// @Tags rack
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Param uuid path string true "UUID of the Task"
// @Param siteId query string true "ID of the Site"
// @Success 200 {object} model.APITask
// @Router /v2/org/{org}/carbide/rack/task/{uuid} [get]
func (gth GetTaskHandler) Handle(c echo.Context) error {
	org, dbUser, ctx, logger, handlerSpan := common.SetupHandler("Task", "Get", c, gth.tracerSpan)
	if handlerSpan != nil {
		defer handlerSpan.End()
	}

	var apiRequest model.APIGetTaskRequest
	if err := common.ValidateKnownQueryParams(c.QueryParams(), apiRequest); err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
	}
	if err := c.Bind(&apiRequest); err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to parse request data", nil)
	}
	if err := apiRequest.Validate(); err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
	}

	if dbUser == nil {
		logger.Error().Msg("invalid User object found in request context")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	ok, err := auth.ValidateOrgMembership(dbUser, org)
	if !ok {
		if err != nil {
			logger.Error().Err(err).Msg("error validating org membership for User in request")
		} else {
			logger.Warn().Msg("could not validate org membership for user, access denied")
		}
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}

	ok = auth.ValidateUserRoles(dbUser, org, nil, auth.ProviderAdminRole)
	if !ok {
		logger.Warn().Msg("user does not have Provider Admin role, access denied")
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "User does not have Provider Admin role with org", nil)
	}

	infrastructureProvider, err := common.GetInfrastructureProviderForOrg(ctx, nil, gth.dbSession, org)
	if err != nil {
		logger.Warn().Err(err).Msg("error getting infrastructure provider for org")
		return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to retrieve Infrastructure Provider for org", nil)
	}

	taskUUID := c.Param("uuid")

	site, err := common.GetSiteFromIDString(ctx, nil, apiRequest.SiteID, gth.dbSession)
	if err != nil {
		if errors.Is(err, common.ErrInvalidID) {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Failed to validate Site specified in request: invalid ID", nil)
		}
		if errors.Is(err, cdb.ErrDoesNotExist) {
			return cutil.NewAPIErrorResponse(c, http.StatusBadRequest, "Site specified in request does not exist", nil)
		}
		logger.Error().Err(err).Msg("error retrieving Site from DB")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve Site specified in request due to DB error", nil)
	}

	if site.InfrastructureProviderID != infrastructureProvider.ID {
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, "Site specified in request doesn't belong to current org's Provider", nil)
	}

	siteConfig := &cdbm.SiteConfig{}
	if site.Config != nil {
		siteConfig = site.Config
	}

	if !siteConfig.RackLevelAdministration {
		logger.Warn().Msg("site does not have Rack Level Administration enabled")
		return cutil.NewAPIErrorResponse(c, http.StatusPreconditionFailed, "Site does not have Rack Level Administration enabled", nil)
	}

	stc, err := gth.scp.GetClientByID(site.ID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve client for Site", nil)
	}

	rlaRequest := &rlav1.GetTasksByIDsRequest{
		TaskIds: []*rlav1.UUID{{Id: taskUUID}},
	}

	workflowOptions := tClient.StartWorkflowOptions{
		ID:                       fmt.Sprintf("task-get-%s", taskUUID),
		WorkflowIDReusePolicy:    temporalEnums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		WorkflowIDConflictPolicy: temporalEnums.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
		WorkflowExecutionTimeout: cutil.WorkflowExecutionTimeout,
		TaskQueue:                queue.SiteTaskQueue,
	}

	ctx, cancel := context.WithTimeout(ctx, cutil.WorkflowContextTimeout)
	defer cancel()

	we, err := stc.ExecuteWorkflow(ctx, workflowOptions, "GetTaskByID", rlaRequest)
	if err != nil {
		logger.Error().Err(err).Msg("failed to execute GetTaskByID workflow")
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to get Task details", nil)
	}

	var rlaResponse rlav1.GetTasksByIDsResponse
	err = we.Get(ctx, &rlaResponse)
	if err != nil {
		var timeoutErr *tp.TimeoutError
		if errors.As(err, &timeoutErr) || err == context.DeadlineExceeded || ctx.Err() != nil {
			return common.TerminateWorkflowOnTimeOut(c, logger, stc, fmt.Sprintf("task-get-%s", taskUUID), err, "Task", "GetTaskByID")
		}
		code, err := common.UnwrapWorkflowError(err)
		logger.Error().Err(err).Msg("failed to get result from GetTaskByID workflow")
		return cutil.NewAPIErrorResponse(c, code, fmt.Sprintf("Failed to get Task details: %s", err), nil)
	}

	tasks := rlaResponse.GetTasks()
	if len(tasks) == 0 {
		return cutil.NewAPIErrorResponse(c, http.StatusNotFound, "Task not found", nil)
	}

	apiTask := model.NewAPITask(tasks[0])

	logger.Info().Msg("finishing API handler")

	return c.JSON(http.StatusOK, apiTask)
}
