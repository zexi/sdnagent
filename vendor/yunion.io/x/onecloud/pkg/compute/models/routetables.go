// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package models

import (
	"context"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/gotypes"
	"yunion.io/x/pkg/util/compare"
	"yunion.io/x/sqlchemy"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/lockman"
	"yunion.io/x/onecloud/pkg/cloudcommon/validators"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

type SRouteTableManager struct {
	db.SStatusInfrasResourceBaseManager
	db.SExternalizedResourceBaseManager
	SVpcResourceBaseManager
}

var RouteTableManager *SRouteTableManager

func init() {
	RouteTableManager = &SRouteTableManager{
		SStatusInfrasResourceBaseManager: db.NewStatusInfrasResourceBaseManager(
			SRouteTable{},
			"route_tables_tbl",
			"route_table",
			"route_tables",
		),
	}
	RouteTableManager.SetVirtualObject(RouteTableManager)
}

type SRouteTable struct {
	db.SStatusInfrasResourceBase
	db.SExternalizedResourceBase
	SVpcResourceBase `create:"required"`

	Type   string       `width:"16" charset:"ascii" nullable:"false" list:"user"`
	Routes *api.SRoutes `list:"user" update:"user" create:"required"`
}

// VPC虚拟路由表列表
func (man *SRouteTableManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.RouteTableListInput,
) (*sqlchemy.SQuery, error) {
	var err error

	q, err = man.SStatusInfrasResourceBaseManager.ListItemFilter(ctx, q, userCred, query.StatusInfrasResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SStatusInfrasResourceBaseManager.ListItemFilter")
	}

	q, err = man.SExternalizedResourceBaseManager.ListItemFilter(ctx, q, userCred, query.ExternalizedResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SExternalizedResourceBaseManager.ListItemFilter")
	}

	q, err = man.SVpcResourceBaseManager.ListItemFilter(ctx, q, userCred, query.VpcFilterListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SVpcResourceBaseManager.ListItemFilter")
	}

	if len(query.Type) > 0 {
		q = q.In("type", query.Type)
	}

	return q, nil
}

func (man *SRouteTableManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.RouteTableListInput,
) (*sqlchemy.SQuery, error) {
	var err error

	q, err = man.SStatusInfrasResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.StatusInfrasResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SStatusInfrasResourceBaseManager.OrderByExtraFields")
	}

	q, err = man.SVpcResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.VpcFilterListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SVpcResourceBaseManager.OrderByExtraFields")
	}

	return q, nil
}

func (man *SRouteTableManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	var err error

	q, err = man.SInfrasResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}
	q, err = man.SVpcResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}

	return q, httperrors.ErrNotFound
}

func (man *SRouteTableManager) validateRoutes(data *jsonutils.JSONDict, update bool) (*jsonutils.JSONDict, error) {
	routes := api.SRoutes{}
	routesV := validators.NewStructValidator("routes", &routes)
	if update {
		routesV.Optional(true)
	}
	err := routesV.Validate(data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (man *SRouteTableManager) ValidateCreateData(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	ownerId mcclient.IIdentityProvider,
	query jsonutils.JSONObject,
	input api.RouteTableCreateInput,
) (api.RouteTableCreateInput, error) {
	_, err := man.validateRoutes(jsonutils.Marshal(input).(*jsonutils.JSONDict), false)
	if err != nil {
		return input, errors.Wrap(err, "validateRoutes")
	}
	_, input.VpcResourceInput, err = ValidateVpcResourceInput(userCred, input.VpcResourceInput)
	if err != nil {
		return input, errors.Wrap(err, "ValidateVpcResourceInput")
	}
	input.StatusInfrasResourceBaseCreateInput, err = man.SStatusInfrasResourceBaseManager.ValidateCreateData(ctx, userCred, ownerId, query, input.StatusInfrasResourceBaseCreateInput)
	if err != nil {
		return input, errors.Wrap(err, "SStatusInfrasResourceBaseManager.ValidateCreateData")
	}
	return input, nil
}

func (rt *SRouteTable) AllowPerformPurge(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) bool {
	return db.IsAdminAllowPerform(userCred, rt, "purge")
}

func (rt *SRouteTable) PerformPurge(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	err := rt.ValidateDeleteCondition(ctx)
	if err != nil {
		return nil, err
	}
	provider := rt.GetCloudprovider()
	if provider != nil {
		if provider.GetEnabled() {
			return nil, httperrors.NewInvalidStatusError("Cannot purge route_table on enabled cloud provider")
		}
	}
	err = rt.RealDelete(ctx, userCred)
	return nil, err
}

func (rt *SRouteTable) RealDelete(ctx context.Context, userCred mcclient.TokenCredential) error {
	return rt.SStatusInfrasResourceBase.Delete(ctx, userCred)
}

func (rt *SRouteTable) ValidateUpdateData(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	input api.RouteTableUpdateInput,
) (api.RouteTableUpdateInput, error) {
	_, err := RouteTableManager.validateRoutes(jsonutils.Marshal(input).(*jsonutils.JSONDict), true)
	if err != nil {
		return input, errors.Wrap(err, "RouteTableManager.validateRoutes")
	}
	input.StatusInfrasResourceBaseUpdateInput, err = rt.SStatusInfrasResourceBase.ValidateUpdateData(ctx, userCred, query, input.StatusInfrasResourceBaseUpdateInput)
	if err != nil {
		return input, errors.Wrap(err, "SStatusInfrasResourceBase.ValidateUpdateData")
	}
	return input, nil
}

func (rt *SRouteTable) AllowPerformAddRoutes(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data *jsonutils.JSONDict) bool {
	return rt.IsOwner(userCred) || db.IsAdminAllowPerform(userCred, rt, "add-routes")
}

func (rt *SRouteTable) AllowPerformDelRoutes(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data *jsonutils.JSONDict) bool {
	return rt.AllowPerformAddRoutes(ctx, userCred, query, data)
}

// PerformAddRoutes patches acl entries by adding then deleting the specified acls.
// This is intended mainly for command line operations.
func (rt *SRouteTable) PerformAddRoutes(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data *jsonutils.JSONDict) (*jsonutils.JSONDict, error) {
	var routes api.SRoutes
	if rt.Routes != nil {
		routes_ := gotypes.DeepCopy(rt.Routes).(*api.SRoutes)
		routes = *routes_
	}
	{
		adds := api.SRoutes{}
		addsV := validators.NewStructValidator("routes", &adds)
		addsV.Optional(true)
		err := addsV.Validate(data)
		if err != nil {
			return nil, err
		}
		for _, add := range adds {
			found := false
			for _, route := range routes {
				if route.Cidr == add.Cidr {
					found = true
					break
				}
			}
			if !found {
				routes = append(routes, add)
			}
		}
	}
	_, err := db.Update(rt, func() error {
		rt.Routes = &routes
		return nil
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (rt *SRouteTable) PerformDelRoutes(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data *jsonutils.JSONDict) (*jsonutils.JSONDict, error) {
	var routes api.SRoutes
	if rt.Routes != nil {
		routes_ := gotypes.DeepCopy(rt.Routes).(*api.SRoutes)
		routes = *routes_
	}
	{
		cidrs := []string{}
		err := data.Unmarshal(&cidrs, "cidrs")
		if err != nil {
			return nil, httperrors.NewInputParameterError("unmarshaling cidrs failed: %s", err)
		}
		for _, cidr := range cidrs {
			for i := len(routes) - 1; i >= 0; i-- {
				route := routes[i]
				if route.Type == "system" {
					continue
				}
				if route.Cidr == cidr {
					routes = append(routes[:i], routes[i+1:]...)
					break
				}
			}
		}
	}
	_, err := db.Update(rt, func() error {
		rt.Routes = &routes
		return nil
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (rt *SRouteTable) GetExtraDetails(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, isList bool) (api.RouteTableDetails, error) {
	return api.RouteTableDetails{}, nil
}

func (manager *SRouteTableManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.RouteTableDetails {
	rows := make([]api.RouteTableDetails, len(objs))

	virtRows := manager.SStatusInfrasResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	vpcRows := manager.SVpcResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)

	for i := range rows {
		rows[i] = api.RouteTableDetails{
			StatusInfrasResourceBaseDetails: virtRows[i],
			VpcResourceInfo:                 vpcRows[i],
		}
	}

	return rows
}

func (man *SRouteTableManager) SyncRouteTables(ctx context.Context, userCred mcclient.TokenCredential, vpc *SVpc, cloudRouteTables []cloudprovider.ICloudRouteTable, provider *SCloudprovider) ([]SRouteTable, []cloudprovider.ICloudRouteTable, compare.SyncResult) {
	lockman.LockClass(ctx, man, db.GetLockClassKey(man, userCred))
	defer lockman.ReleaseClass(ctx, man, db.GetLockClassKey(man, userCred))

	localRouteTables := make([]SRouteTable, 0)
	remoteRouteTables := make([]cloudprovider.ICloudRouteTable, 0)
	syncResult := compare.SyncResult{}

	dbRouteTables := []SRouteTable{}
	if err := db.FetchModelObjects(man, man.Query().Equals("vpc_id", vpc.Id), &dbRouteTables); err != nil {
		syncResult.Error(err)
		return nil, nil, syncResult
	}
	removed := make([]SRouteTable, 0)
	commondb := make([]SRouteTable, 0)
	commonext := make([]cloudprovider.ICloudRouteTable, 0)
	added := make([]cloudprovider.ICloudRouteTable, 0)
	if err := compare.CompareSets(dbRouteTables, cloudRouteTables, &removed, &commondb, &commonext, &added); err != nil {
		syncResult.Error(err)
		return nil, nil, syncResult
	}

	for i := 0; i < len(removed); i += 1 {
		err := removed[i].syncRemoveCloudRouteTable(ctx, userCred)
		if err != nil {
			syncResult.DeleteError(err)
		} else {
			syncResult.Delete()
		}
	}

	for i := 0; i < len(commondb); i += 1 {
		err := commondb[i].SyncWithCloudRouteTable(ctx, userCred, vpc, commonext[i], provider.GetOwnerId())
		if err != nil {
			syncResult.UpdateError(err)
			continue
		}
		syncMetadata(ctx, userCred, &commondb[i], commonext[i])
		localRouteTables = append(localRouteTables, commondb[i])
		remoteRouteTables = append(remoteRouteTables, commonext[i])
		syncResult.Update()
	}

	for i := 0; i < len(added); i += 1 {
		routeTableNew, err := man.insertFromCloud(ctx, userCred, vpc, added[i], provider)
		if err != nil {
			syncResult.AddError(err)
			continue
		}
		syncMetadata(ctx, userCred, routeTableNew, added[i])
		localRouteTables = append(localRouteTables, *routeTableNew)
		remoteRouteTables = append(remoteRouteTables, added[i])
		syncResult.Add()
	}
	return localRouteTables, remoteRouteTables, syncResult
}

func (man *SRouteTableManager) newRouteTableFromCloud(userCred mcclient.TokenCredential, vpc *SVpc, cloudRouteTable cloudprovider.ICloudRouteTable) (*SRouteTable, error) {
	routes := api.SRoutes{}
	{
		cloudRoutes, err := cloudRouteTable.GetIRoutes()
		if err != nil {
			return nil, err
		}
		for _, cloudRoute := range cloudRoutes {
			route := &api.SRoute{
				Type:        cloudRoute.GetType(),
				Cidr:        cloudRoute.GetCidr(),
				NextHopType: cloudRoute.GetNextHopType(),
				NextHopId:   cloudRoute.GetNextHop(),
			}
			routes = append(routes, route)
		}
	}
	routeTable := &SRouteTable{
		Type:   cloudRouteTable.GetType(),
		Routes: &routes,
	}
	routeTable.VpcId = vpc.Id
	{
		basename := routeTableBasename(cloudRouteTable.GetName(), vpc.Name)
		newName, err := db.GenerateName(man, userCred, basename)
		if err != nil {
			return nil, err
		}
		routeTable.Name = newName
	}
	// routeTable.ManagerId = vpc.ManagerId
	routeTable.ExternalId = cloudRouteTable.GetGlobalId()
	routeTable.Description = cloudRouteTable.GetDescription()
	// routeTable.ProjectId = userCred.GetProjectId()
	// routeTable.DomainId = userCred.GetProjectDomainId()
	routeTable.SetModelManager(man, routeTable)
	return routeTable, nil
}

func routeTableBasename(name, vpcName string) string {
	if name != "" {
		return name
	} else if vpcName != "" {
		return "rtbl-" + vpcName
	} else {
		return "rtbl"
	}
}

func (man *SRouteTableManager) insertFromCloud(ctx context.Context, userCred mcclient.TokenCredential, vpc *SVpc, cloudRouteTable cloudprovider.ICloudRouteTable, provider *SCloudprovider) (*SRouteTable, error) {
	routeTable, err := man.newRouteTableFromCloud(userCred, vpc, cloudRouteTable)
	if err != nil {
		return nil, err
	}
	if err := man.TableSpec().Insert(ctx, routeTable); err != nil {
		return nil, err
	}
	SyncCloudDomain(userCred, routeTable, provider.GetOwnerId())
	db.OpsLog.LogEvent(routeTable, db.ACT_CREATE, routeTable.GetShortDesc(ctx), userCred)
	return routeTable, nil
}

func (self *SRouteTable) syncRemoveCloudRouteTable(ctx context.Context, userCred mcclient.TokenCredential) error {
	lockman.LockObject(ctx, self)
	defer lockman.ReleaseObject(ctx, self)

	err := self.ValidateDeleteCondition(ctx)
	if err != nil {
		return err
	}
	err = self.RealDelete(ctx, userCred)
	return err
}

func (self *SRouteTable) SyncWithCloudRouteTable(ctx context.Context, userCred mcclient.TokenCredential, vpc *SVpc, cloudRouteTable cloudprovider.ICloudRouteTable, syncOwnerId mcclient.IIdentityProvider) error {
	man := self.GetModelManager().(*SRouteTableManager)
	routeTable, err := man.newRouteTableFromCloud(userCred, vpc, cloudRouteTable)
	if err != nil {
		return err
	}
	diff, err := db.UpdateWithLock(ctx, self, func() error {
		// self.CloudregionId = routeTable.CloudregionId
		self.VpcId = vpc.Id
		self.Type = routeTable.Type
		self.Routes = routeTable.Routes
		return nil
	})
	if err != nil {
		return err
	}

	SyncCloudDomain(userCred, self, syncOwnerId)

	db.OpsLog.LogSyncUpdate(self, diff, userCred)
	return nil
}

func (self *SRouteTable) getVpc() (*SVpc, error) {
	val, err := VpcManager.FetchById(self.VpcId)
	if err != nil {
		log.Errorf("VpcManager.FetchById fail %s", err)
		return nil, err
	}
	return val.(*SVpc), nil
}

func (self *SRouteTable) getRegion() (*SCloudregion, error) {
	vpc, err := self.getVpc()
	if err != nil {
		return nil, err
	}
	return vpc.GetRegion()
}

func (self *SRouteTable) getCloudProviderInfo() SCloudProviderInfo {
	region, _ := self.getRegion()
	provider := self.GetCloudprovider()
	return MakeCloudProviderInfo(region, nil, provider)
}