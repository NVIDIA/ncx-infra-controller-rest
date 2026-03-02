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

package vpc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.temporal.io/sdk/client"

	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"
	cdbm "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/model"
	cdbp "github.com/nvidia/bare-metal-manager-rest/db/pkg/db/paginator"

	cwm "github.com/nvidia/bare-metal-manager-rest/workflow/internal/metrics"
	sc "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/client/site"
	"github.com/nvidia/bare-metal-manager-rest/workflow/pkg/queue"
	"github.com/nvidia/bare-metal-manager-rest/workflow/pkg/util"

	cwssaws "github.com/nvidia/bare-metal-manager-rest/workflow-schema/schema/site-agent/workflows/v1"

	cwutil "github.com/nvidia/bare-metal-manager-rest/common/pkg/util"
)

// ManageVpc is an activity wrapper for managing VPC lifecycle that allows
// injecting DB access
type ManageVpc struct {
	dbSession      *cdb.Session
	siteClientPool *sc.ClientPool
	tc             client.Client
}

// Activity functions

// CreateVpcViaSiteAgent is a Temporal activity that create a VPC in Site Controller via Site agent
func (mv ManageVpc) CreateVpcViaSiteAgent(ctx context.Context, siteID uuid.UUID, vpcID uuid.UUID) error {
	logger := log.With().Str("Activity", "CreateVpcViaSiteAgent").Str("VPC ID", vpcID.String()).
		Str("Site ID", siteID.String()).Logger()

	logger.Info().Msg("starting activity")

	vpcDAO := cdbm.NewVpcDAO(mv.dbSession)

	vpc, err := vpcDAO.GetByID(ctx, nil, vpcID, nil)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve VPC from DB by ID")
		return err
	}

	if vpc.SiteID != siteID {
		logger.Error().Msg("VPC does not belong to specified Site")
		return fmt.Errorf("VPC does not belong to specified Site")
	}

	logger.Info().Msg("retrieved VPC from DB")

	tc, err := mv.siteClientPool.GetClientByID(siteID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return err
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "site-vpc-create-" + vpcID.String(),
		TaskQueue: queue.SiteTaskQueue,
	}

	transactionID := &cwssaws.TransactionID{
		ResourceId: vpcID.String(),
		Timestamp:  timestamppb.Now(),
	}

	createVpcRequest := &cwssaws.CreateVPCRequest{
		VpcId: &cwssaws.UUID{Value: vpcID.String()},
		// configuring name here untill all
		// sites are updated with Vpc lables changes
		Name:                 vpc.Name,
		TenantOrganizationId: vpc.Org,
	}

	we, err := tc.ExecuteWorkflow(ctx, workflowOptions, "CreateVPC",
		// Workflow arguments
		// Transaction ID
		transactionID,
		// VPC ID
		createVpcRequest,
	)

	status := cdbm.VpcStatusProvisioning
	statusMessage := "initiated VPC provisioning via Site Agent"

	if err != nil {
		status = cdbm.VpcStatusError
		statusMessage = "failed to initiate VPC provisioning via Site Agent"
	}

	_ = mv.updateVpcStatusInDB(ctx, nil, vpcID, &status, &statusMessage)

	if err != nil {
		logger.Error().Err(err).Msg("failed to trigger site agent create VPC workflow")
		return err
	}

	logger.Info().Str("Workflow ID", we.GetID()).Msg("triggered Site agent workflow to create VPC")

	logger.Info().Msg("completed activity")

	return nil
}

// DeleteVpcViaSiteAgent is a Temporal activity that delete a VPC in Site Controller via Site agent
func (mv ManageVpc) DeleteVpcViaSiteAgent(ctx context.Context, siteID uuid.UUID, vpcID uuid.UUID) error {
	logger := log.With().Str("Activity", "DeleteVpcViaSiteAgent").Str("VPC ID", vpcID.String()).
		Str("Site ID", siteID.String()).Logger()

	logger.Info().Msg("starting activity")

	vpcDAO := cdbm.NewVpcDAO(mv.dbSession)

	vpc, err := vpcDAO.GetByID(ctx, nil, vpcID, nil)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve VPC from DB by ID")
		return err
	}

	if vpc.SiteID != siteID {
		logger.Warn().Msg("cannot initiate deletion via Site Agent as VPC does not belong to specified Site")
		return fmt.Errorf("VPC does not belong to specified Site")
	}

	if vpc.ControllerVpcID == nil {
		logger.Warn().Msg("cannot initiate deletion via Site Agent as VPC does not have controller ID set")
		// Return an error to schedule retry, once VPC create call update or inventory is received, controller ID will be populated
		return fmt.Errorf("VPC does not have controller ID set")
	}

	logger.Info().Msg("retrieved VPC from DB")

	tc, err := mv.siteClientPool.GetClientByID(siteID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return err
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "site-vpc-delete-" + vpcID.String(),
		TaskQueue: queue.SiteTaskQueue,
	}

	transactionID := &cwssaws.TransactionID{
		ResourceId: vpcID.String(),
		Timestamp:  timestamppb.Now(),
	}

	deleteVpcRequest := &cwssaws.DeleteVPCRequest{
		Id: &cwssaws.UUID{Value: vpc.ControllerVpcID.String()},
	}

	we, err := tc.ExecuteWorkflow(ctx, workflowOptions, "DeleteVPC",
		// Workflow arguments
		// Transaction ID
		transactionID,
		// VPC ID
		deleteVpcRequest,
	)

	if err != nil {
		logger.Error().Err(err).Msg("failed to trigger site agent delete VPC workflow")
		return err
	}

	status := cdbm.VpcStatusDeleting
	statusMessage := "Deletion request was sent to the Site"
	_ = mv.updateVpcStatusInDB(ctx, nil, vpcID, &status, &statusMessage)

	logger.Info().Str("Workflow ID", we.GetID()).Msg("triggered Site agent workflow to delete VPC")

	logger.Info().Msg("completed activity")

	return nil
}

// UpdateVpcInDB updates the VPC in the DB from data pushed by Site Controller
func (mv ManageVpc) UpdateVpcInDB(ctx context.Context, transactionID *cwssaws.TransactionID, vpcInfo *cwssaws.VPCInfo) error {
	logger := log.With().Str("Activity", "UpdateVpcInDB").Str("VPC ID", transactionID.ResourceId).Logger()

	logger.Info().Msg("starting activity")

	vpcDAO := cdbm.NewVpcDAO(mv.dbSession)

	vpcID, err := uuid.Parse(transactionID.ResourceId)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse VPC ID from transaction ID")
		return err
	}

	vpc, err := vpcDAO.GetByID(ctx, nil, vpcID, nil)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			logger.Error().Err(err).Msg("could not find VPC from DB by resource ID specified in Site agent transaction ID")
		} else {
			logger.Error().Err(err).Msg("failed to retrieve VPC from DB by resource ID specified in Site agent transaction ID")
		}

		return err
	}

	logger.Info().Msg("retrieved VPC from DB")

	// Start a db tx
	tx, err := cdb.BeginTx(ctx, mv.dbSession, &sql.TxOptions{})
	if err != nil {
		logger.Error().Err(err).Msg("failed to start transaction")
		return err
	}

	var status *string
	var statusMessage *string

	if vpcInfo.Status == cwssaws.WorkflowStatus_WORKFLOW_STATUS_SUCCESS {
		if vpcInfo.ObjectStatus == cwssaws.ObjectStatus_OBJECT_STATUS_CREATED {
			status = cdb.GetStrPtr(cdbm.VpcStatusReady)
			statusMessage = cdb.GetStrPtr("VPC successfully provisioned on Site")

			// Controller VPC ID must be extracted/saved
			if vpcInfo.Vpc != nil && vpcInfo.Vpc.Id != nil {
				controllerVpcID, serr := uuid.Parse(vpcInfo.Vpc.Id.Value)
				if serr != nil {
					logger.Error().Err(serr).Msg("failed to parse Site Controller VPC ID from VPC Info")
					terr := tx.Rollback()
					if terr != nil {
						logger.Error().Err(terr).Msg("failed to rollback transaction")
					}
					return serr
				}

				// Initialized Network virtualization type
				networkVirtualizationType := vpc.NetworkVirtualizationType
				if vpcInfo.Vpc.NetworkVirtualizationType != nil && vpc.NetworkVirtualizationType != nil && vpcInfo.Vpc.NetworkVirtualizationType.String() != *vpc.NetworkVirtualizationType {
					networkVirtualizationType = cdb.GetStrPtr(vpcInfo.Vpc.NetworkVirtualizationType.String())
				}

				// Set default `EthernetVirtualizer`
				if networkVirtualizationType == nil || *networkVirtualizationType == "" {
					networkVirtualizationType = cdb.GetStrPtr(cdbm.VpcEthernetVirtualizer)
				}

				// Save controller VPC ID
				_, serr = vpcDAO.Update(ctx, nil, cdbm.VpcUpdateInput{VpcID: vpcID, NetworkVirtualizationType: networkVirtualizationType, ControllerVpcID: &controllerVpcID})
				if serr != nil {
					logger.Error().Err(serr).Msg("failed to update Controller VPC ID in DB")
					terr := tx.Rollback()
					if terr != nil {
						logger.Error().Err(terr).Msg("failed to rollback transaction")
					}
					return serr
				}
			} else {
				errMsg := "controller VPC ID is missing from object creation success response"
				logger.Error().Msg(errMsg)
				terr := tx.Rollback()
				if terr != nil {
					logger.Error().Err(terr).Msg("failed to rollback transaction")
				}
				return errors.New(errMsg)
			}
		} else if vpcInfo.ObjectStatus == cwssaws.ObjectStatus_OBJECT_STATUS_DELETED {
			status = cdb.GetStrPtr(cdbm.VpcStatusDeleting)
			statusMessage = cdb.GetStrPtr("Deletion has been initiated on Site")
		}
	} else if vpcInfo.Status == cwssaws.WorkflowStatus_WORKFLOW_STATUS_FAILURE {
		status = cdb.GetStrPtr(cdbm.VpcStatusError)
		statusMessage = cdb.GetStrPtr(vpcInfo.StatusMsg)

		// If the VPC is being deleted, keep the Deleting status, it'll be purged by inventory if needed
		if vpc.Status == cdbm.VpcStatusDeleting && statusMessage != nil {
			// Log the error but keep the Deleting status
			status = cdb.GetStrPtr(cdbm.VpcStatusDeleting)
		}
	}

	if status != nil {
		err = mv.updateVpcStatusInDB(ctx, tx, vpcID, status, statusMessage)
		if err != nil {
			logger.Error().Err(err).Msg("failed to update VPC status detail in DB")
			terr := tx.Rollback()
			if terr != nil {
				logger.Error().Err(terr).Msg("failed to rollback transaction")
			}
			return err
		}
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		logger.Error().Err(err).Msg("error committing transaction to DB")
		return err
	}

	logger.Info().Msg("retrieved VPC from DB")

	return nil
}

// UpdateVpcsInDB is a Temporal activity that takes a collection of VPC data pushed by Site Agent and updates the DB
func (mv ManageVpc) UpdateVpcsInDB(ctx context.Context, siteID uuid.UUID, vpcInventory *cwssaws.VPCInventory) ([]cwm.InventoryObjectLifecycleEvent, error) {
	logger := log.With().Str("Activity", "UpdateVpcsInDB").Str("Site ID", siteID.String()).Logger()

	logger.Info().Msg("starting activity")

	// Initialize metrics tracking variables
	vpcLifecycleEvents := []cwm.InventoryObjectLifecycleEvent{}

	stDAO := cdbm.NewSiteDAO(mv.dbSession)

	site, err := stDAO.GetByID(ctx, nil, siteID, nil, false)
	if err != nil {
		if err == cdb.ErrDoesNotExist {
			logger.Warn().Err(err).Msg("received VPC inventory for unknown or deleted Site")
		} else {
			logger.Error().Err(err).Msg("failed to retrieve Site from DB")
		}
		return nil, err
	}

	// Get temporal client for specified Site
	tc, err := mv.siteClientPool.GetClientByID(siteID)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve Temporal client for Site")
		return nil, err
	}

	if vpcInventory.InventoryStatus == cwssaws.InventoryStatus_INVENTORY_STATUS_FAILED {
		logger.Warn().Msg("received failed inventory status from Site Agent, skipping inventory processing")
		return nil, errors.New(vpcInventory.StatusMsg)
	}

	vpcDAO := cdbm.NewVpcDAO(mv.dbSession)
	sdDAO := cdbm.NewStatusDetailDAO(mv.dbSession)

	existingVpcs, _, err := vpcDAO.GetAll(ctx, nil, cdbm.VpcFilterInput{SiteIDs: []uuid.UUID{site.ID}}, cdbp.PageInput{Limit: cdb.GetIntPtr(cdbp.TotalLimit)}, nil)
	if err != nil {
		logger.Error().Err(err).Msg("failed to get VPCs for Site from DB")
		return nil, err
	}

	// Construct a map of Controller VPC ID to VPC
	existingVpcIDMap := make(map[string]*cdbm.Vpc)
	existingVpcCtrlIDMap := make(map[string]*cdbm.Vpc)

	for _, vpc := range existingVpcs {
		curVPC := vpc
		existingVpcIDMap[vpc.ID.String()] = &curVPC
		if vpc.ControllerVpcID != nil {
			existingVpcCtrlIDMap[vpc.ControllerVpcID.String()] = &curVPC
		}
	}

	reportedVpcIDMap := map[uuid.UUID]bool{}

	if vpcInventory.InventoryPage != nil {
		logger.Info().Msgf("Received VPC inventory page: %d of %d, page size: %d, total count: %d",
			vpcInventory.InventoryPage.CurrentPage, vpcInventory.InventoryPage.TotalPages,
			vpcInventory.InventoryPage.PageSize, vpcInventory.InventoryPage.TotalItems)

		for _, strId := range vpcInventory.InventoryPage.ItemIds {
			id, serr := uuid.Parse(strId)
			if serr != nil {
				logger.Error().Err(serr).Str("ID", strId).Msg("failed to parse VPC ID from inventory page")
				continue
			}
			reportedVpcIDMap[id] = true
		}
	}

	// Prepare a map of ID -> propagation status
	// so we can quickly attach it to the object
	// when need to perform the update query.
	vpcPropagationStatus := map[string]*cdbm.NetworkSecurityGroupPropagationDetails{}
	for _, propStatus := range vpcInventory.NetworkSecurityGroupPropagations {
		vpcPropagationStatus[propStatus.Id] = &cdbm.NetworkSecurityGroupPropagationDetails{NetworkSecurityGroupPropagationObjectStatus: propStatus}
		logger.Debug().Str("Controller VPC ID", propStatus.Id).Msg("propagation details cached for VPC")
	}

	// Iterate through VPC Inventory and update DB
	for _, controllerVpc := range vpcInventory.Vpcs {
		slogger := logger.With().Str("VPC Controller ID", controllerVpc.Id.Value).Logger()

		sitePropagationStatus := vpcPropagationStatus[controllerVpc.Id.Value]
		logger.Debug().Str("Controller VPC ID", controllerVpc.Id.Value).Msgf("cached propagation status for VPC %+v", sitePropagationStatus)

		vpc, ok := existingVpcCtrlIDMap[controllerVpc.Id.Value]
		if !ok {
			// Check if the VPC is found by ID (controllerVpc.Name == cloudVpc.ID)
			vpc, ok = existingVpcIDMap[controllerVpc.Name]
			if ok {
				existingVpcCtrlIDMap[controllerVpc.Id.Value] = vpc
			}
		}

		if vpc == nil {
			logger.Warn().Str("Controller VPC ID", controllerVpc.Id.Value).Msg("VPC does not have a record in DB, possibly created directly on Site")
			continue
		}

		reportedVpcIDMap[vpc.ID] = true

		// Reset missing flag if necessary
		var isMissingOnSite *bool
		if vpc.IsMissingOnSite {
			isMissingOnSite = cdb.GetBoolPtr(false)
		}

		// Populate controller VPC ID if necessary
		var controllerVpcID *uuid.UUID
		if vpc.ControllerVpcID == nil {
			ctrlID, serr := uuid.Parse(controllerVpc.Id.Value)
			if serr != nil {
				slogger.Error().Err(serr).Msg("failed to parse VPC Controller ID, not a valid UUID")
				continue
			}
			controllerVpcID = &ctrlID
		}

		// Initialized Network virtualization type
		var networkVirtualizationType *string
		// If the VPC in the DB has Network Virtualization Type, but Site reported different one then update it
		if controllerVpc.NetworkVirtualizationType != nil &&
			(vpc.NetworkVirtualizationType == nil || (vpc.NetworkVirtualizationType != nil &&
				controllerVpc.NetworkVirtualizationType.String() != *vpc.NetworkVirtualizationType)) {
			networkVirtualizationType = cdb.GetStrPtr(controllerVpc.NetworkVirtualizationType.String())
		}

		needsUpdate := isMissingOnSite != nil ||
			controllerVpcID != nil ||
			networkVirtualizationType != nil ||
			!util.NetworkSecurityGroupPropagationDetailsEqual(vpc.NetworkSecurityGroupPropagationDetails, sitePropagationStatus)

		if needsUpdate {
			// If the VPC in the DB has propagation details but the site reported no propagation details
			// then we should clear it in the DB.  Passing along the nil to the Update call would
			// just ignore the field.
			if vpc.NetworkSecurityGroupPropagationDetails != nil && sitePropagationStatus == nil {
				vpc, err = vpcDAO.Clear(ctx, nil, cdbm.VpcClearInput{
					VpcID:                                  vpc.ID,
					NetworkSecurityGroupPropagationDetails: true,
				})
				if err != nil {
					slogger.Error().Err(err).Msg("failed to clear NetworkSecurityGroupPropagationDetails for VPC in DB")
					continue
				}
			}

			_, serr := vpcDAO.Update(ctx, nil, cdbm.VpcUpdateInput{VpcID: vpc.ID, NetworkSecurityGroupID: controllerVpc.NetworkSecurityGroupId, NetworkSecurityGroupPropagationDetails: sitePropagationStatus, NetworkVirtualizationType: networkVirtualizationType, ControllerVpcID: controllerVpcID, IsMissingOnSite: isMissingOnSite})
			if serr != nil {
				slogger.Error().Err(serr).Msg("failed to update missing on Site flag/controller VPC ID in DB")
				continue
			}
		}

		// If VPC is not in Deleting state, then update status to Ready
		if vpc.Status != cdbm.VpcStatusDeleting && vpc.Status != cdbm.VpcStatusReady {
			err = mv.updateVpcStatusInDB(ctx, nil, vpc.ID, cdb.GetStrPtr(cdbm.VpcStatusReady), cdb.GetStrPtr("VPC has been re-detected on Site"))
			if err != nil {
				slogger.Error().Err(err).Msg("failed to update VPC status detail in DB")
			}
		}

		// Verify if VPC's metadata update required, if yes trigger `UpdateVPC` workflow
		if controllerVpc.Metadata != nil {
			triggerVpcMetadataUpdate := false

			if vpc.Name != controllerVpc.Metadata.Name {
				triggerVpcMetadataUpdate = true
			}

			if vpc.Description != nil && *vpc.Description != controllerVpc.Metadata.Description {
				triggerVpcMetadataUpdate = true
			}

			if controllerVpc.Metadata.Labels != nil && vpc.Labels != nil {
				if len(vpc.Labels) != len(controllerVpc.Metadata.Labels) {
					triggerVpcMetadataUpdate = true
				} else {
					// Verify if each label matches with Vpc in cloud
					for _, label := range controllerVpc.Metadata.Labels {
						if label != nil {
							// case1: Key not found
							_, ok := vpc.Labels[label.Key]
							if !ok {
								triggerVpcMetadataUpdate = true
								break
							}

							// case2: Value isn't matching
							if label.Value != nil {
								if vpc.Labels[label.Key] != *label.Value {
									triggerVpcMetadataUpdate = true
									break
								}
							}
						}
					}
				}
			}

			// Trigger update Vpc metadata workflow
			if triggerVpcMetadataUpdate {
				_ = mv.UpdateVpcMetadata(ctx, siteID, tc, vpc.ID, controllerVpc)
			}
		}
	}

	// Populate list of VPCs that were not found
	vpcsToDelete := []*cdbm.Vpc{}

	// If inventory paging is enabled, we only need to do this once and we do it on the last page
	if vpcInventory.InventoryPage == nil || vpcInventory.InventoryPage.TotalPages == 0 || (vpcInventory.InventoryPage.CurrentPage == vpcInventory.InventoryPage.TotalPages) {
		for _, vpc := range existingVpcIDMap {
			found := false

			_, found = reportedVpcIDMap[vpc.ID]
			if !found && vpc.ControllerVpcID != nil {
				// Additional check if controller VPC ID != VPC ID
				_, found = reportedVpcIDMap[*vpc.ControllerVpcID]
			}

			if !found {
				// The VPC was not found in the VPC Inventory, so add it to list of VPCs to potentially terminate
				vpcsToDelete = append(vpcsToDelete, vpc)
			}
		}
	}

	// Loop through VPCs for deletion
	for _, vpc := range vpcsToDelete {
		slogger := logger.With().Str("VPC ID", vpc.ID.String()).Logger()

		// If the VPC was already being deleted, we can proceed with removing it from the DB
		if vpc.Status == cdbm.VpcStatusDeleting {
			serr := vpcDAO.DeleteByID(ctx, nil, vpc.ID)
			if serr != nil {
				slogger.Error().Err(serr).Msg("failed to delete VPC from DB")
			} else {
				// Add VPC ID to deletedVpcIDs list
				vpcLifecycleEvents = append(vpcLifecycleEvents, cwm.InventoryObjectLifecycleEvent{
					ObjectID: vpc.ID,
					Deleted:  cdb.GetTimePtr(time.Now()),
				})
			}
		} else if vpc.ControllerVpcID != nil {
			// Was this created within inventory receipt interval? If so, we may be processing an older inventory
			if time.Since(vpc.Created) < cwutil.InventoryReceiptInterval {
				continue
			}

			status := cdbm.VpcStatusError
			statusMessage := "VPC is missing on Site"

			// Leave orderBy as nil as the result is sorted by created timestamp by default
			if status == vpc.Status {
				latestsd, _, serr := sdDAO.GetAllByEntityID(ctx, nil, vpc.ID.String(), nil, cdb.GetIntPtr(1), nil)
				if serr != nil {
					slogger.Error().Err(serr).Msg("failed to retrieve latest Status Detail for VPC")
					continue
				}

				if len(latestsd) > 0 && latestsd[0].Message != nil && *latestsd[0].Message == statusMessage {
					continue
				}
			}

			// Set isMissingOnSite flag to true and update status, user can decide on deletion
			_, serr := vpcDAO.Update(ctx, nil, cdbm.VpcUpdateInput{VpcID: vpc.ID, IsMissingOnSite: cdb.GetBoolPtr(true)})
			if serr != nil {
				slogger.Error().Err(serr).Msg("failed to set missing on Site flag in DB")
				continue
			}

			serr = mv.updateVpcStatusInDB(ctx, nil, vpc.ID, &status, &statusMessage)
			if serr != nil {
				slogger.Error().Err(serr).Msg("failed to update status and/or create Status Detail in DB")
			}
		}
	}

	return vpcLifecycleEvents, nil
}

// updateVpcStatusInDB is helper function to write VPC updates to DB
func (mv ManageVpc) updateVpcStatusInDB(ctx context.Context, tx *cdb.Tx, vpcID uuid.UUID, status *string, statusMessage *string) error {
	if status != nil {
		vpcDAO := cdbm.NewVpcDAO(mv.dbSession)

		_, err := vpcDAO.Update(ctx, tx, cdbm.VpcUpdateInput{VpcID: vpcID, Status: status})
		if err != nil {
			return err
		}

		statusDetailDAO := cdbm.NewStatusDetailDAO(mv.dbSession)
		_, err = statusDetailDAO.CreateFromParams(ctx, tx, vpcID.String(), *status, statusMessage)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateVpcMetadata is a Temporal activity that will trigger an update of an vpc's metadata
// if they are found out of sync with the cloud.
func (mv ManageVpc) UpdateVpcMetadata(ctx context.Context, siteID uuid.UUID, tc client.Client, vpcID uuid.UUID, controllerVpc *cwssaws.Vpc) error {
	logger := log.With().Str("Activity", "UpdateVpcMetadata").Str("Site ID", siteID.String()).Str("VPC ID", vpcID.String()).Logger()

	logger.Info().Msg("starting activity")

	vpcDAO := cdbm.NewVpcDAO(mv.dbSession)
	vpc, err := vpcDAO.GetByID(ctx, nil, vpcID, nil)
	if err != nil {
		logger.Error().Err(err).Msg("failed to retrieve VPC from DB by ID")
		return err
	}

	logger.Info().Msg("retrieved VPC from DB")

	description := ""
	if vpc.Description != nil {
		description = *vpc.Description
	}

	// Prepare the labels for the metadata of the carbide call.
	labels := []*cwssaws.Label{}
	for k, v := range vpc.Labels {
		labels = append(labels, &cwssaws.Label{
			Key:   k,
			Value: &v,
		})
	}

	// Build an update request for vpc that needs a sync metadata and call UpdateVpc.
	workflowOptions := client.StartWorkflowOptions{
		ID:        "site-vpc-update-metadata-" + vpcID.String(),
		TaskQueue: queue.SiteTaskQueue,
	}

	// Prepare the config update request workflow object
	updateVpcRequest := &cwssaws.VpcUpdateRequest{
		Id: &cwssaws.VpcId{Value: vpc.ID.String()},
		Metadata: &cwssaws.Metadata{
			Name:        vpc.Name,
			Description: description,
			Labels:      labels,
		},
	}

	we, err := tc.ExecuteWorkflow(ctx, workflowOptions, "UpdateVPC", updateVpcRequest)
	if err != nil {
		logger.Error().Err(err).Str("VPC ID", vpc.ID.String()).Msg("failed to trigger workflow to update VPC Metadata")
	} else {
		logger.Info().Str("Workflow ID", we.GetID()).Msg("triggered workflow to update VPC Metadata")
	}

	logger.Info().Msg("completed activity")

	return nil
}

// NewManageVpc returns a new ManageVpc activity
func NewManageVpc(dbSession *cdb.Session, siteClientPool *sc.ClientPool, tc client.Client) ManageVpc {
	return ManageVpc{
		dbSession:      dbSession,
		siteClientPool: siteClientPool,
		tc:             tc,
	}
}

// ManageVpcLifecycleMetrics is an activity wrapper for managing VPC lifecycle metrics
type ManageVpcLifecycleMetrics struct {
	dbSession            *cdb.Session
	statusTransitionTime *prometheus.GaugeVec
	siteIDNameMap        map[uuid.UUID]string
}

// RecordVpcStatusTransitionMetrics is a Temporal activity that records duration of important status transitions for VPCs
func (mvlm ManageVpcLifecycleMetrics) RecordVpcStatusTransitionMetrics(ctx context.Context, siteID uuid.UUID, vpcLifecycleEvents []cwm.InventoryObjectLifecycleEvent) error {
	logger := log.With().Str("Activity", "RecordVpcStatusTransitionMetrics").Str("Site ID", siteID.String()).Logger()

	logger.Info().Msg("starting activity")

	// Cache site name to avoid repeated DB call
	siteName, ok := mvlm.siteIDNameMap[siteID]
	if !ok {
		siteDAO := cdbm.NewSiteDAO(mvlm.dbSession)
		site, err := siteDAO.GetByID(context.Background(), nil, siteID, nil, false)
		if err != nil {
			logger.Error().Err(err).Str("Site ID", siteID.String()).Msg("failed to retrieve Site from DB")
			return err
		}
		siteName = site.Name
		mvlm.siteIDNameMap[siteID] = siteName
	}

	logger.Info().Int("EventCount", len(vpcLifecycleEvents)).Str("Site Name", siteName).Msg("processing vpc lifecycle events")

	// Get status details for each VPC
	sdDAO := cdbm.NewStatusDetailDAO(mvlm.dbSession)
	metricsRecorded := 0

	for _, event := range vpcLifecycleEvents {
		statusDetails, _, err := sdDAO.GetAllByEntityID(ctx, nil, event.ObjectID.String(), nil, cdb.GetIntPtr(cdbp.TotalLimit), nil)
		if err != nil {
			logger.Error().Err(err).Str("VPC ID", event.ObjectID.String()).Msg("failed to retrieve Status Details for VPC")
			return err
		}

		if event.Created != nil {
			// NOTE: VPC create operation is not tracked in this activity since it is created in synchronous manner and it should never arrive here
			logger.Warn().Str("VPC ID", event.ObjectID.String()).Msg("VPC create operation is not tracked in this activity since it is created in synchronous manner and it should never arrive here")
		} else if event.Deleted != nil {
			// DELETE event: Measure time from Deleting to actual deletion
			// Find the earliest Deleting status (iterate backwards since sorted DESC)
			var deletingStatusDetail *cdbm.StatusDetail
			for i := len(statusDetails) - 1; i >= 0; i-- {
				sd := &statusDetails[i]
				if sd.Status == cdbm.VpcStatusDeleting {
					deletingStatusDetail = sd
					break
				}
			}

			if deletingStatusDetail != nil {
				// Calculate duration from Deleting status to deletion time
				duration := event.Deleted.Sub(deletingStatusDetail.Created)
				// Note: VPC doesn't have VpcStatusDeleted constant, so we use string "Deleted"
				mvlm.statusTransitionTime.WithLabelValues(siteName, cwm.InventoryOperationTypeDelete, cdbm.VpcStatusDeleting, "Deleted").Set(duration.Seconds())
				metricsRecorded++
				logger.Info().
					Str("VPC ID", event.ObjectID.String()).
					Str("Operation", "DELETE").
					Float64("Duration Seconds", duration.Seconds()).
					Msg("recorded vpc lifecycle metric")
			} else {
				logger.Debug().
					Str("VPC ID", event.ObjectID.String()).
					Msg("skipped vpc DELETE metric")
			}
		}
	}

	logger.Info().Int("MetricsRecorded", metricsRecorded).Msg("completed activity")

	return nil
}

// NewManageVpcLifecycleMetrics returns a new ManageVpcLifecycleMetrics activity
func NewManageVpcLifecycleMetrics(reg prometheus.Registerer, dbSession *cdb.Session) ManageVpcLifecycleMetrics {
	lifecycleMetrics := ManageVpcLifecycleMetrics{
		dbSession: dbSession,
		statusTransitionTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: cwm.MetricsNamespace,
				Name:      "vpc_operation_latency_seconds",
				Help:      "Current latency of vpc operations",
			},
			[]string{"site", "operation_type", "from_status", "to_status"}),

		siteIDNameMap: map[uuid.UUID]string{},
	}
	reg.MustRegister(lifecycleMetrics.statusTransitionTime)

	return lifecycleMetrics
}
