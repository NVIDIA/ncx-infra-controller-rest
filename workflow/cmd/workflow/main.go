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

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentryZerolog "github.com/getsentry/sentry-go/zerolog"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	zlogadapter "logur.dev/adapter/zerolog"
	"logur.dev/logur"

	tsdkClient "go.temporal.io/sdk/client"
	tsdkConverter "go.temporal.io/sdk/converter"
	tsdkWorker "go.temporal.io/sdk/worker"

	"go.opentelemetry.io/otel"
	"go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	cdb "github.com/nvidia/bare-metal-manager-rest/db/pkg/db"

	"github.com/nvidia/bare-metal-manager-rest/workflow/internal/config"

	cwm "github.com/nvidia/bare-metal-manager-rest/workflow/internal/metrics"
	cwfh "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/health"
	cwfn "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/namespace"

	sc "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/client/site"

	machineActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/machine"
	machineWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/machine"

	vpcActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/vpc"
	vpcWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/vpc"

	subnetActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/subnet"
	subnetWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/subnet"

	instanceActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/instance"
	instanceWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/instance"

	userActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/user"
	userWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/user"

	siteActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/site"
	siteWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/site"

	sshKeyGroupActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/sshkeygroup"
	sshKeyGroupWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/sshkeygroup"

	ibpActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/infinibandpartition"
	ibpWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/infinibandpartition"

	expectedMachineActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/expectedmachine"
	expectedMachineWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/expectedmachine"

	expectedPowerShelfActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/expectedpowershelf"
	expectedPowerShelfWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/expectedpowershelf"

	expectedSwitchActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/expectedswitch"
	expectedSwitchWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/expectedswitch"

	tenantActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/tenant"
	tenantWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/tenant"

	instanceTypeActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/instancetype"
	instanceTypeWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/instancetype"

	networkSecurityGroupActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/networksecuritygroup"
	networkSecurityGroupWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/networksecuritygroup"

	osImageActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/operatingsystem"
	osImageWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/operatingsystem"

	skuActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/sku"
	skuWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/sku"

	vpcPrefixActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/vpcprefix"
	vpcPrefixWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/vpcprefix"

	vpcPeeringActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/vpcpeering"
	vpcPeeringWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/vpcpeering"

	dpuExtensionServiceActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/dpuextensionservice"
	dpuExtensionServiceWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/dpuextensionservice"

	nvLinkLogicalPartitionActivity "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/activity/nvlinklogicalpartition"
	nvLinkLogicalPartitionWorkflow "github.com/nvidia/bare-metal-manager-rest/workflow/pkg/workflow/nvlinklogicalpartition"
)

const (
	// ZerologMessageFieldName specifies the field name for log message
	ZerologMessageFieldName = "msg"
	// ZerologLevelFieldName specifies the field name for log level
	ZerologLevelFieldName = "type"
)

func main() {
	// Initialize context
	ctx := context.Background()

	// Initialize logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.LevelFieldName = ZerologLevelFieldName
	zerolog.MessageFieldName = ZerologMessageFieldName

	cfg := config.NewConfig()
	defer cfg.Close()

	dbConfig := cfg.GetDBConfig()

	// Initialize DB connection
	dbSession, err := cdb.NewSession(ctx, dbConfig.Host, dbConfig.Port, dbConfig.Name, dbConfig.User, dbConfig.Password, "")
	if err != nil {
		log.Panic().Err(err).Msg("failed to initialize DB session")
	} else {
		defer dbSession.Close()
	}

	// Initializer Temporal client
	// Create the client object just once per process
	log.Info().Msg("creating Temporal client")

	// set up sentry client
	sentryDSN := os.Getenv("SENTRY_DSN")
	if sentryDSN != "" {
		// Initialize Sentry
		err := sentry.Init(sentry.ClientOptions{
			Dsn: sentryDSN,
			BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
				// Modify or filter events before sending them to Sentry
				return event
			},
			Debug:            true,
			AttachStacktrace: true,
		})
		if err != nil {
			log.Error().Err(err).Msg("Sentry initialization failed")
		} else {
			defer sentry.Flush(2 * time.Second)

			// Configure Zerolog to use Sentry as a writer
			sentryWriter, err := sentryZerolog.New(sentryZerolog.Config{
				ClientOptions: sentry.ClientOptions{
					Dsn: sentryDSN,
				},
				Options: sentryZerolog.Options{
					Levels:          []zerolog.Level{zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel},
					WithBreadcrumbs: true,
					FlushTimeout:    3 * time.Second,
				},
			})
			if err != nil {
				log.Error().Err(err).Msg("failed to create Sentry writer")
			} else {
				defer sentryWriter.Close()

				// Use Sentry writer in Zerolog
				log.Logger = zerolog.New(zerolog.MultiLevelWriter(os.Stderr, sentryWriter))
			}
		}
	}

	tLogger := logur.LoggerToKV(zlogadapter.New(zerolog.New(os.Stderr)))
	var tc tsdkClient.Client

	tcfg, err := cfg.GetTemporalConfig()
	if err != nil {
		log.Panic().Err(err).Msg("failed to get Temporal config")
	}

	var tInterceptors []interceptor.ClientInterceptor

	if cfg.GetTracingEnabled() {
		otelInterceptor, err := opentelemetry.NewTracingInterceptor(opentelemetry.TracerOptions{TextMapPropagator: otel.GetTextMapPropagator()})
		if err != nil {
			log.Panic().Err(err).Msg("unable to get otelInterceptor")
		}
		tInterceptors = append(tInterceptors, otelInterceptor)
	}

	tc, err = tsdkClient.NewLazyClient(tsdkClient.Options{
		HostPort:  fmt.Sprintf("%v:%v", tcfg.Host, tcfg.Port),
		Namespace: tcfg.Namespace,
		ConnectionOptions: tsdkClient.ConnectionOptions{
			TLS: tcfg.ClientTLSCfg,
		},
		DataConverter: tsdkConverter.NewCompositeDataConverter(
			tsdkConverter.NewNilPayloadConverter(),
			tsdkConverter.NewByteSlicePayloadConverter(),
			tsdkConverter.NewProtoJSONPayloadConverterWithOptions(tsdkConverter.ProtoJSONPayloadConverterOptions{
				AllowUnknownFields: true,
			}),
			tsdkConverter.NewProtoPayloadConverter(),
			tsdkConverter.NewJSONPayloadConverter(),
		),
		// Interceptors: tInterceptors,
		Logger: tLogger,
	})

	if err != nil {
		log.Panic().Err(err).Msg("failed to create Temporal client")
	} else {
		defer tc.Close()
	}

	w := tsdkWorker.New(tc, tcfg.Queue, tsdkWorker.Options{
		WorkflowPanicPolicy:              tsdkWorker.FailWorkflow,
		MaxConcurrentActivityTaskPollers: 10,
		MaxConcurrentWorkflowTaskPollers: 10,
	})

	siteClientPool := sc.NewClientPool(tcfg)

	log.Info().Str("Temporal Namespace", tcfg.Namespace).Msg("registering workflow and activities")

	// Register workflows
	if tcfg.Namespace == cwfn.CloudNamespace {
		// Workflows triggered by Cloud services
		// VPC workflows
		w.RegisterWorkflow(vpcWorkflow.CreateVpc)
		w.RegisterWorkflow(vpcWorkflow.DeleteVpc)

		// Subnet workflows
		w.RegisterWorkflow(subnetWorkflow.CreateSubnet)
		w.RegisterWorkflow(subnetWorkflow.DeleteSubnet)

		// Instance workflows
		w.RegisterWorkflow(instanceWorkflow.CreateInstance)
		w.RegisterWorkflow(instanceWorkflow.DeleteInstance)
		w.RegisterWorkflow(instanceWorkflow.RebootInstance)

		// ExpectedMachine workflows (none under Cloud namespace)

		// User workflows
		w.RegisterWorkflow(userWorkflow.UpdateUserFromNGC)
		w.RegisterWorkflow(userWorkflow.UpdateUserFromNGCWithAuxiliaryID)

		// Site workflows
		w.RegisterWorkflow(siteWorkflow.DeleteSiteComponents)
		w.RegisterWorkflow(siteWorkflow.MonitorHealthForAllSites)
		w.RegisterWorkflow(siteWorkflow.CheckHealthForAllSites)
		w.RegisterWorkflow(siteWorkflow.MonitorTemporalCertExpirationForAllSites)
		w.RegisterWorkflow(siteWorkflow.MonitorSiteTemporalNamespaces)

		// SSHKeyGroup workflows
		w.RegisterWorkflow(sshKeyGroupWorkflow.SyncSSHKeyGroup)
		w.RegisterWorkflow(sshKeyGroupWorkflow.DeleteSSHKeyGroup)

		// InfiniBandPartition workflows
		w.RegisterWorkflow(ibpWorkflow.CreateInfiniBandPartition)
		w.RegisterWorkflow(ibpWorkflow.DeleteInfiniBandPartition)
	} else if tcfg.Namespace == cwfn.SiteNamespace {
		// Workflows triggered by Site Agent
		// Machine Workflows
		w.RegisterWorkflow(machineWorkflow.UpdateMachineInventory)

		// VPC workflows
		w.RegisterWorkflow(vpcWorkflow.UpdateVpcInfo)
		w.RegisterWorkflow(vpcWorkflow.UpdateVpcInventory)

		// Subnet workflows
		w.RegisterWorkflow(subnetWorkflow.UpdateSubnetInfo)
		w.RegisterWorkflow(subnetWorkflow.UpdateSubnetInventory)

		// Instance workflows
		w.RegisterWorkflow(instanceWorkflow.UpdateInstanceInfo)
		w.RegisterWorkflow(instanceWorkflow.UpdateInstanceInventory)
		w.RegisterWorkflow(instanceWorkflow.UpdateInstanceRebootInfo)

		// Site workflows
		w.RegisterWorkflow(siteWorkflow.UpdateAgentCertExpiry)

		// SSHKeyGroup workflows
		w.RegisterWorkflow(sshKeyGroupWorkflow.UpdateSSHKeyGroupInfo)
		w.RegisterWorkflow(sshKeyGroupWorkflow.UpdateSSHKeyGroupInventory)

		// InfiniBandPartition workflows
		w.RegisterWorkflow(ibpWorkflow.UpdateInfiniBandPartitionInfo)
		w.RegisterWorkflow(ibpWorkflow.UpdateInfiniBandPartitionInventory)

		// Tenant workflow
		w.RegisterWorkflow(tenantWorkflow.UpdateTenantInventory)

		// InstanceType workflow
		w.RegisterWorkflow(instanceTypeWorkflow.UpdateInstanceTypeInventory)

		// NetworkSecurityGroup workflow
		w.RegisterWorkflow(networkSecurityGroupWorkflow.UpdateNetworkSecurityGroupInventory)

		// OS Image workflow
		w.RegisterWorkflow(osImageWorkflow.UpdateOsImageInventory)

		// VPC Prefix workflow
		w.RegisterWorkflow(vpcPrefixWorkflow.UpdateVpcPrefixInventory)

		// VPC Peering workflow
		w.RegisterWorkflow(vpcPeeringWorkflow.UpdateVpcPeeringInventory)

		// ExpectedMachine workflow
		w.RegisterWorkflow(expectedMachineWorkflow.UpdateExpectedMachineInventory)

		// ExpectedPowerShelf workflow
		w.RegisterWorkflow(expectedPowerShelfWorkflow.UpdateExpectedPowerShelfInventory)

		// ExpectedSwitch workflow
		w.RegisterWorkflow(expectedSwitchWorkflow.UpdateExpectedSwitchInventory)

		// SKU workflow
		w.RegisterWorkflow(skuWorkflow.UpdateSkuInventory)

		// DPU Extension Service workflow
		w.RegisterWorkflow(dpuExtensionServiceWorkflow.UpdateDpuExtensionServiceInventory)

		// NVLink Logical Partition workflow
		w.RegisterWorkflow(nvLinkLogicalPartitionWorkflow.UpdateNVLinkLogicalPartitionInventory)
	}

	// Register activities
	// Common activities
	machineManager := machineActivity.NewManageMachine(dbSession, siteClientPool)
	w.RegisterActivity(&machineManager)

	vpcManager := vpcActivity.NewManageVpc(dbSession, siteClientPool, tc)
	w.RegisterActivity(&vpcManager)

	subnetManager := subnetActivity.NewManageSubnet(dbSession, siteClientPool, tc)
	w.RegisterActivity(&subnetManager)

	instanceManager := instanceActivity.NewManageInstance(dbSession, siteClientPool, tc, cfg)
	w.RegisterActivity(&instanceManager)

	siteManager := siteActivity.NewManageSite(dbSession, siteClientPool, tc, cfg)
	w.RegisterActivity(&siteManager)

	sshKeyGroupManager := sshKeyGroupActivity.NewManageSSHKeyGroup(dbSession, siteClientPool)
	w.RegisterActivity(&sshKeyGroupManager)

	ibpManager := ibpActivity.NewManageInfiniBandPartition(dbSession, siteClientPool)
	w.RegisterActivity(&ibpManager)

	tenantManager := tenantActivity.NewManageTenant(dbSession, siteClientPool)
	w.RegisterActivity(&tenantManager)

	instanceTypeManager := instanceTypeActivity.NewManageInstanceType(dbSession, siteClientPool)
	w.RegisterActivity(&instanceTypeManager)

	networkSecurityGroupManager := networkSecurityGroupActivity.NewManageNetworkSecurityGroup(dbSession, siteClientPool)
	w.RegisterActivity(&networkSecurityGroupManager)

	osImageManager := osImageActivity.NewManageOsImage(dbSession, siteClientPool)
	w.RegisterActivity(&osImageManager)

	vpcPrefixManager := vpcPrefixActivity.NewManageVpcPrefix(dbSession, siteClientPool)
	w.RegisterActivity(&vpcPrefixManager)

	vpcPeeringManager := vpcPeeringActivity.NewManageVpcPeering(dbSession, siteClientPool)
	w.RegisterActivity(&vpcPeeringManager)

	// ExpectedMachine activities
	expectedMachineManager := expectedMachineActivity.NewManageExpectedMachine(dbSession, siteClientPool)
	w.RegisterActivity(&expectedMachineManager)

	// ExpectedPowerShelf activities
	expectedPowerShelfManager := expectedPowerShelfActivity.NewManageExpectedPowerShelf(dbSession, siteClientPool)
	w.RegisterActivity(&expectedPowerShelfManager)

	// ExpectedSwitch activities
	expectedSwitchManager := expectedSwitchActivity.NewManageExpectedSwitch(dbSession, siteClientPool)
	w.RegisterActivity(&expectedSwitchManager)

	// SKU activities
	skuManager := skuActivity.NewManageSku(dbSession, siteClientPool)
	w.RegisterActivity(&skuManager)

	// DPU Extension Service activities
	dpuExtensionServiceManager := dpuExtensionServiceActivity.NewManageDpuExtensionService(dbSession, siteClientPool)
	w.RegisterActivity(&dpuExtensionServiceManager)

	// NVLink Logical Partition activities
	nvLinkLogicalPartitionManager := nvLinkLogicalPartitionActivity.NewManageNVLinkLogicalPartition(dbSession, siteClientPool)
	w.RegisterActivity(&nvLinkLogicalPartitionManager)

	if tcfg.Namespace == cwfn.CloudNamespace {
		// User activities
		userManager := userActivity.NewManageUser(dbSession, cfg)
		w.RegisterActivity(&userManager)
	}

	// Serve health endpoint
	hconfig := cfg.GetHealthzConfig()
	if hconfig.Enabled {
		go func() {
			log.Info().Msg("starting health check API server")
			http.HandleFunc("/healthz", cwfh.StatusHandler)
			http.HandleFunc("/readyz", cwfh.StatusHandler)

			serr := http.ListenAndServe(hconfig.GetListenAddr(), nil)
			if serr != nil {
				log.Panic().Err(serr).Msg("failed to start health check server")
			}
		}()
	}

	mconfig := cfg.GetMetricsConfig()
	if mconfig.Enabled {
		// Serve Prometheus metrics
		go func() {
			log.Info().Msg("starting Prometheus metrics server")

			reg := prometheus.NewRegistry()
			reg.MustRegister(collectors.NewGoCollector())

			// Register core metrics
			cm := cwm.NewCoreMetrics(reg)
			// TODO: Set version here when available
			cm.Info.With(prometheus.Labels{"version": "unknown", "namespace": tcfg.Namespace}).Set(1)

			if tcfg.Namespace == cwfn.SiteNamespace {
				// Register common inventory metrics activity
				inventoryMetricsManager := cwm.NewManageInventoryMetrics(reg, dbSession)
				w.RegisterActivity(&inventoryMetricsManager)

				// Register inventory operation metrics activity
				vpcLifecycleMetricsManager := vpcActivity.NewManageVpcLifecycleMetrics(reg, dbSession)
				w.RegisterActivity(&vpcLifecycleMetricsManager)

				subnetLifecycleMetricsManager := subnetActivity.NewManageSubnetLifecycleMetrics(reg, dbSession)
				w.RegisterActivity(&subnetLifecycleMetricsManager)

				instanceLifecycleMetricsManager := instanceActivity.NewManageInstanceLifecycleMetrics(reg, dbSession)
				w.RegisterActivity(&instanceLifecycleMetricsManager)
			}

			promHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg})

			http.Handle("/metrics", promHandler)
			serr := http.ListenAndServe(mconfig.GetListenAddr(), nil)
			if serr != nil {
				log.Panic().Err(serr).Msg("failed to start Prometheus metrics server")
			}
		}()
	}

	// Start listening to the Task Queue
	log.Info().Str("Temporal Namespace", tcfg.Namespace).Msg("starting Temporal worker")
	err = w.Run(tsdkWorker.InterruptCh())
	if err != nil {
		log.Panic().Err(err).Str("Temporal Namespace", tcfg.Namespace).Msg("failed to start worker")
	}

	// Trigger cron workflow
	if tcfg.Namespace == cwfn.CloudNamespace {
		_, err := siteWorkflow.ExecuteMonitorHealthForAllSitesWorkflow(ctx, tc)
		if err != nil {
			log.Error().Err(err).Msg("failed to trigger Site Health Monitor workflow")
		}

		// Trigger MonitorTemporalCertExpirationForAllSites
		_, err = siteWorkflow.ExecuteMonitorTemporalCertExpirationForAllSites(ctx, tc)
		if err != nil {
			log.Error().Err(err).Msg("failed to trigger Temporal Cert Expiration Monitor workflow")
		}
		// NOTE: This will stay disabled until Site Agent is ready
		// _, err = siteWorkflow.ExecuteCheckHealthForAllSitesWorkflow(ctx, tc)
		// if err != nil {
		// 	log.Error().Err(err).Msg("failed to trigger Site Agent Health Monitor workflow")
		// }

		// Trigger MonitorSiteTemporalNamespaces
		_, err = siteWorkflow.ExecuteMonitorSiteTemporalNamespaces(ctx, tc)
		if err != nil {
			log.Error().Err(err).Msg("failed to trigger Monitor Site Temporal Namespaces workflow")
		}
	}
	// NOTE: Log messages past this point do not show up in the log output
}
