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
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/labstack/echo/v4"

	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"

	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/handler/util/common"
	"github.com/nvidia/bare-metal-manager-rest/api/pkg/api/model"
	auth "github.com/nvidia/bare-metal-manager-rest/auth/pkg/authorization"
	cutil "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
)

// GetUserHandler is an API Handler to return information about the current user
type GetUserHandler struct {
	dbSession  *cdb.Session
	tracerSpan *cutil.TracerSpan
}

// NewGetUserHandler creates and returns a new handler
func NewGetUserHandler(dbSession *cdb.Session) GetUserHandler {
	return GetUserHandler{
		dbSession:  dbSession,
		tracerSpan: cutil.NewTracerSpan(),
	}
}

// Handle godoc
// @Summary Return information about the current user
// @Description Get basic information about the user making the request
// @Tags user
// @Accept */*
// @Produce json
// @Security ApiKeyAuth
// @Param org path string true "Name of NGC organization"
// @Success 200 {object} model.APIUser
// @Router /v2/org/{org}/carbide/user/current [get]
func (guh GetUserHandler) Handle(c echo.Context) error {
	// Get context
	ctx := c.Request().Context()

	// Get org
	org := c.Param("orgName")

	// Initialize logger
	logger := log.With().Str("Model", "User").Str("Handler", "Get").Str("Org", org).Logger()

	logger.Info().Msg("started API handler")

	// Create a child span and set the attributes for current request
	newctx, handlerSpan := guh.tracerSpan.CreateChildInContext(ctx, "GetUserHandler", logger)
	if handlerSpan != nil {
		// Set newly created span context as a current context
		ctx = newctx

		defer handlerSpan.End()

		guh.tracerSpan.SetAttribute(handlerSpan, attribute.String("org", org), logger)
	}

	dbUser, logger, err := common.GetUserAndEnrichLogger(c, logger, guh.tracerSpan, handlerSpan)
	if err != nil {
		return cutil.NewAPIErrorResponse(c, http.StatusInternalServerError, "Failed to retrieve current user", nil)
	}

	// Validate org
	ok, err := auth.ValidateOrgMembership(dbUser, org)
	if !ok {
		if err != nil {
			logger.Error().Err(err).Msg("error validating org membership for User in request")
		} else {
			logger.Warn().Msg("could not validate org membership for user, access denied")
		}
		return cutil.NewAPIErrorResponse(c, http.StatusForbidden, fmt.Sprintf("Failed to validate membership for org: %s", org), nil)
	}

	apiUser := model.NewAPIUserFromDBUser(*dbUser)

	logger.Info().Msg("finishing API handler")

	return c.JSON(http.StatusOK, apiUser)
}
