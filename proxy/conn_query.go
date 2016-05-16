// Copyright 2016 The kingshard Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package server

import (
	"github.com/compasses/mysql-load-balancer/backend"
	"github.com/flike/kingshard/core/golog"
)

func (c *ClientConn) GetBackendConn(nodeName string) (co *backend.BackendConn, err error) {
	node := c.proxy.GetNode(nodeName)
	return c.getBackendConn(node, false)
}

func (c *ClientConn) getBackendConn(n *backend.Node, fromSlave bool) (co *backend.BackendConn, err error) {
	co, err = n.GetMasterConn()
	if err != nil {
		golog.Error("Server", "getBackendConn from master", err.Error(), 0)
		co, err = n.GetSlaveConn()
		if err != nil {
			golog.Error("Server", "getBackendConn from slave failed, no connection available!", err.Error(), 0)
		}
	}
	return
}
