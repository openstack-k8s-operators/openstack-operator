/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package functional

// NOTE: Comprehensive multi-cluster RabbitMQ finalizer tests have been implemented
// in openstackdataplanenodeset_rabbitmq_finalizer_test.go
//
// The tests cover:
//
// CORE FUNCTIONALITY:
// 1. Incremental Node Deployments
//    - 3-node deployment with incremental updates using ansibleLimit
//    - Finalizer added only after ALL nodes are updated
//
// 2. Multi-NodeSet Shared User Management
//    - Independent finalizers per nodeset on shared RabbitMQ user
//    - User protected until all nodesets remove their finalizers
//    - Deletion of one nodeset doesn't affect others
//
// 3. RabbitMQ User Credential Rotation
//    - Switch from old user to new user during rolling update
//    - Finalizer moves from old to new user after all nodes updated
//    - Safe credential rotation without service interruption
//
// ADVANCED SCENARIOS:
// 4. Multi-Service RabbitMQ Cluster Management
//    - Multiple services (Nova, Neutron, Ironic) using different clusters
//    - Service-specific finalizers (nodeset.os/{hash}-{service})
//    - Independent lifecycle management per service
//
// 5. Deployment Timing and Secret Changes
//    - Deployment completion time vs creation time validation
//    - Secret changes during active deployment (resets tracking)
//    - Multiple deployment scenarios and timing edge cases
//
// See openstackdataplanenodeset_rabbitmq_finalizer_test.go for full implementation.
