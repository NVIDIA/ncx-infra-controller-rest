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

package managers

import (
	"net/http"

	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/machinevalidation"

	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/metadata"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/bootstrap"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/carbide"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/dpuextensionservice"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/expectedmachine"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/expectedpowershelf"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/expectedswitch"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/health"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/infinibandpartition"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/instance"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/instancetype"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/machine"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/managerapi"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/networksecuritygroup"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/nvlinklogicalpartition"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/operatingsystem"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/rla"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/sku"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/sshkeygroup"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/subnet"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/tenant"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/vpc"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/vpcprefix"
	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/managers/workflow"

	"github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/datatypes/elektratypes"

	computils "github.com/nvidia/bare-metal-manager-rest/site-agent/pkg/components/utils"
)

// NewAPIHandlers - handle new api
func NewAPIHandlers() {
	managerapi.ManagerHdl = managerapi.ManagerAPI{
		// Add all the Managers here
		Orchestrator:           &workflow.API{},
		VPC:                    &vpc.API{},
		VpcPrefix:              &vpcprefix.API{},
		Subnet:                 &subnet.API{},
		Instance:               &instance.API{},
		Machine:                &machine.API{},
		Carbide:                &carbide.API{},
		Bootstrap:              &bootstrap.BoostrapAPI{},
		Health:                 &health.API{},
		SSHKeyGroup:            &sshkeygroup.API{},
		InfiniBandPartition:    &infinibandpartition.API{},
		Tenant:                 &tenant.API{},
		OperatingSystem:        &operatingsystem.API{},
		MachineValidation:      &machinevalidation.API{},
		InstanceType:           &instancetype.API{},
		NetworkSecurityGroup:   &networksecuritygroup.API{},
		ExpectedMachine:        &expectedmachine.API{},
		ExpectedPowerShelf:     &expectedpowershelf.API{},
		ExpectedSwitch:         &expectedswitch.API{},
		SKU:                    &sku.API{},
		DpuExtensionService:    &dpuextensionservice.API{},
		NVLinkLogicalPartition: &nvlinklogicalpartition.API{},
		RLA:                    &rla.API{},
	}
}

// NewInstance - new instance with the parent datastruct
func NewInstance(superforge *elektratypes.Elektra) (*Manager, error) {
	NewAPIHandlers()
	ManagerAccess = &Manager{
		Data: &managerapi.ManagerData{
			EB: superforge,
		},
		API: &managerapi.ManagerHdl,
		Conf: &managerapi.ManagerConf{
			EB: superforge.Conf,
		},
	}
	ManagerAccess.NewInstance()
	return ManagerAccess, nil
}

// NewInstance - instantiates all the managers
func (Managers *Manager) NewInstance() {
	// Instantiate all the managers here
	Managers.Orchestrator()
	Managers.VPC()
	Managers.VpcPrefix()
	Managers.Subnet()
	Managers.Instance()
	Managers.Carbide()
	Managers.Machine()
	Managers.Bootstrap()
	Managers.Health()
	Managers.SSHKeyGroup()
	Managers.InfiniBandPartition()
	Managers.Tenant()
	Managers.OperatingSystem()
	Managers.MachineValidation()
	Managers.InstanceType()
	Managers.NetworkSecurityGroup()
	Managers.ExpectedMachine()
	Managers.ExpectedPowerShelf()
	Managers.ExpectedSwitch()
	Managers.SKU()
	Managers.DpuExtensionService()
	Managers.NVLinkLogicalPartition()
	Managers.RLA()
}

// Init - initialize all the mgrs
func (Managers *Manager) Init() {
	ManagerAccess.Data.EB.Log.Info().Msg("Managers: Initializing all the managers")
	// register version metric (build_version, build_date)
	versionGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "elektra_site_agent",
		Name:      "version",
		Help:      "version of the elektra_site_agent",
	}, []string{"build_version", "build_date"})
	prometheus.MustRegister(versionGauge)
	// set the value once, since it does not change
	versionGauge.WithLabelValues(metadata.Version, metadata.BuildTime).Set(1)
	// register health status metric
	prometheus.MustRegister(
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "elektra_site_agent",
			Name:      "health_status",
			Help:      "health status of the elektra_site_agent",
		},
			func() float64 {
				return float64(ManagerAccess.Data.EB.HealthStatus.Load())
			}))
	ManagerAccess.Data.EB.HealthStatus.Store(uint64(computils.CompUnhealthy))

	Managers.Orchestrator().Init()
	Managers.Carbide().Init()
	Managers.Bootstrap().Init()
	Managers.VPC().Init()
	Managers.VpcPrefix().Init()
	Managers.Subnet().Init()
	Managers.Instance().Init()
	Managers.Health().Init()
	Managers.SSHKeyGroup().Init()
	Managers.InfiniBandPartition().Init()
	Managers.Tenant().Init()
	Managers.OperatingSystem().Init()
	Managers.MachineValidation().Init()
	Managers.InstanceType().Init()
	Managers.NetworkSecurityGroup().Init()
	Managers.ExpectedMachine().Init()
	Managers.ExpectedPowerShelf().Init()
	Managers.ExpectedSwitch().Init()
	Managers.SKU().Init()
	Managers.DpuExtensionService().Init()
	Managers.NVLinkLogicalPartition().Init()
	Managers.RLA().Init()

}

// Start - start the mgrs
func (Managers *Manager) Start() {
	go StartMetricServer()
	StartHTTPServer()
	ManagerAccess.Data.EB.Log.Info().Msg("Managers: Starting all the managers")
	Managers.Carbide().Start()
	Managers.Bootstrap().Start()
	Managers.Orchestrator().Start()
	Managers.RLA().Start()
}

// StartMetricServer - Start serving Metric Server
func StartMetricServer() {
	log.Info().Msgf("Beginning to serve on port %v", ManagerAccess.Conf.EB.MetricsPort)
	http.Handle("/metrics", promhttp.Handler())
	port := ":" + ManagerAccess.Conf.EB.MetricsPort
	http.ListenAndServe(port, nil)
}
