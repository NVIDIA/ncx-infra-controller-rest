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

package workflow

import (
	"errors"
	"reflect"

	"github.com/NVIDIA/ncx-infra-controller-rest/site-agent/pkg/components/utils"
)

// AddWorkflow - Register all the resource workflow functions here
func (w *API) AddWorkflow(wf interface{}) {
	// Register the workflow here
	ManagerAccess.Data.EB.Log.Info().Str("Function", utils.GetFunctionName(wf)).Msg("Workflow: Registering the workflow")
	ManagerAccess.Data.EB.Managers.Workflow.WorkflowFunctions = append(
		ManagerAccess.Data.EB.Managers.Workflow.WorkflowFunctions,
		wf,
	)
}

// Invoke all the resource workflow functions here
func (w *API) Invoke() error {
	// Invoke the workflow here
	ManagerAccess.Data.EB.Log.Info().Msg("Workflow: Invoking the workflow")
	for _, wf := range ManagerAccess.Data.EB.Managers.Workflow.WorkflowFunctions {
		if err := reflect.ValueOf(wf).Call([]reflect.Value{}); err != nil {
			for _, verr := range err {
				// Add the Iszero utility function here later
				if verr.Interface() != reflect.ValueOf(reflect.Type(nil)) {
					ManagerAccess.Data.EB.Log.Error().Str("Function", utils.GetFunctionName(wf)).Msg("Workflow: Failed to invoke the workflow")
					return errors.New("invoke error")
				}
				ManagerAccess.Data.EB.Log.Info().Str("Function", utils.GetFunctionName(wf)).Msg("Workflow: Invoked the workflow")
			}
		}

	}
	return nil
}
