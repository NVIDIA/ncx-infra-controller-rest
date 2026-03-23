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

package model

import (
	"context"
	"database/sql"
	"time"

	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db"
	"github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/db/paginator"
	stracer "github.com/NVIDIA/ncx-infra-controller-rest/db/pkg/tracer"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

const (
	// IpxeTemplateRelationName is the relation name for the IpxeTemplate model
	IpxeTemplateRelationName = "IpxeTemplate"
	// IpxeTemplateOrderByCreated is the field name for ordering by created timestamp
	IpxeTemplateOrderByCreated = "created"
	// ipxeTemplateOrderByUpdated is the field name for ordering by updated timestamp
	ipxeTemplateOrderByUpdated = "updated"
	// IpxeTemplateOrderByName is the field name for ordering by name
	IpxeTemplateOrderByName = "name"
	// IpxeTemplateOrderByDefault is the default field for ordering
	IpxeTemplateOrderByDefault = IpxeTemplateOrderByName

	// IpxeTemplateScopeInternal represents an internal-only template
	IpxeTemplateScopeInternal = "internal"
	// IpxeTemplateScopePublic represents a public template
	IpxeTemplateScopePublic = "public"
)

var (
	// IpxeTemplateOrderByFields is a list of valid order by fields for the IpxeTemplate model
	IpxeTemplateOrderByFields = []string{IpxeTemplateOrderByCreated, ipxeTemplateOrderByUpdated, IpxeTemplateOrderByName}
	// IpxeTemplateRelatedEntities is a list of valid relation by fields for the IpxeTemplate model
	IpxeTemplateRelatedEntities = map[string]bool{
		SiteRelationName: true,
	}
)

// IpxeTemplate represents an iPXE script template propagated from bare-metal-manager-core
type IpxeTemplate struct {
	bun.BaseModel `bun:"table:ipxe_template,alias:it"`

	ID                uuid.UUID `bun:"id,pk,type:uuid"`
	SiteID            uuid.UUID `bun:"site_id,type:uuid,notnull"`
	Site              *Site     `bun:"rel:belongs-to,join:site_id=id"`
	Name              string    `bun:"name,notnull"`
	Template          string    `bun:"template,notnull,default:''"`
	RequiredParams    []string  `bun:"required_params,type:text[],default:'{}'"`
	ReservedParams    []string  `bun:"reserved_params,type:text[],default:'{}'"`
	RequiredArtifacts []string  `bun:"required_artifacts,type:text[],default:'{}'"`
	Scope             string    `bun:"scope,notnull"`
	Created           time.Time `bun:"created,nullzero,notnull,default:current_timestamp"`
	Updated           time.Time `bun:"updated,nullzero,notnull,default:current_timestamp"`
}

// IpxeTemplateCreateInput are input parameters for the Create method
type IpxeTemplateCreateInput struct {
	SiteID            uuid.UUID
	Name              string
	Template          string
	RequiredParams    []string
	ReservedParams    []string
	RequiredArtifacts []string
	Scope             string
}

// IpxeTemplateUpdateInput are input parameters for the Update method
type IpxeTemplateUpdateInput struct {
	ID                uuid.UUID
	Template          string
	RequiredParams    []string
	ReservedParams    []string
	RequiredArtifacts []string
	Scope             string
}

// IpxeTemplateFilterInput are input parameters for the filter/GetAll method
type IpxeTemplateFilterInput struct {
	SiteIDs []uuid.UUID
	Names   []string
	Scopes  []string
}

var _ bun.BeforeAppendModelHook = (*IpxeTemplate)(nil)

// BeforeAppendModel is a hook called before the model is appended to the query
func (it *IpxeTemplate) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		it.Created = db.GetCurTime()
		it.Updated = db.GetCurTime()
	case *bun.UpdateQuery:
		it.Updated = db.GetCurTime()
	}
	return nil
}

var _ bun.BeforeCreateTableHook = (*IpxeTemplate)(nil)

// BeforeCreateTable is a hook called before the table is created
func (it *IpxeTemplate) BeforeCreateTable(ctx context.Context, query *bun.CreateTableQuery) error {
	query.ForeignKey(`("site_id") REFERENCES "site" ("id")`)
	return nil
}

// IpxeTemplateDAO is an interface for interacting with the IpxeTemplate model
type IpxeTemplateDAO interface {
	// Create inserts a new iPXE template row
	Create(ctx context.Context, tx *db.Tx, input IpxeTemplateCreateInput) (*IpxeTemplate, error)
	// Update updates an existing iPXE template row
	Update(ctx context.Context, tx *db.Tx, input IpxeTemplateUpdateInput) (*IpxeTemplate, error)
	// Delete removes an iPXE template row by ID
	Delete(ctx context.Context, tx *db.Tx, id uuid.UUID) error
	// GetAll returns all rows matching the filter and page inputs
	GetAll(ctx context.Context, tx *db.Tx, filter IpxeTemplateFilterInput, page paginator.PageInput) ([]IpxeTemplate, int, error)
	// Get returns the row for the specified ID
	Get(ctx context.Context, tx *db.Tx, id uuid.UUID) (*IpxeTemplate, error)
	// GetBySiteAndName returns the row for the specified site and template name
	GetBySiteAndName(ctx context.Context, tx *db.Tx, siteID uuid.UUID, name string) (*IpxeTemplate, error)
}

// IpxeTemplateSQLDAO is an implementation of the IpxeTemplateDAO interface
type IpxeTemplateSQLDAO struct {
	dbSession *db.Session
	IpxeTemplateDAO
	tracerSpan *stracer.TracerSpan
}

// Create inserts a new IpxeTemplate from the given parameters
func (itd IpxeTemplateSQLDAO) Create(ctx context.Context, tx *db.Tx, input IpxeTemplateCreateInput) (*IpxeTemplate, error) {
	ctx, span := itd.tracerSpan.CreateChildInCurrentContext(ctx, "IpxeTemplateDAO.Create")
	if span != nil {
		defer span.End()
	}

	it := &IpxeTemplate{
		ID:                uuid.New(),
		SiteID:            input.SiteID,
		Name:              input.Name,
		Template:          input.Template,
		RequiredParams:    input.RequiredParams,
		ReservedParams:    input.ReservedParams,
		RequiredArtifacts: input.RequiredArtifacts,
		Scope:             input.Scope,
	}

	_, err := db.GetIDB(tx, itd.dbSession).NewInsert().Model(it).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return itd.Get(ctx, tx, it.ID)
}

// Get returns an IpxeTemplate by ID
// Returns db.ErrDoesNotExist if the record is not found
func (itd IpxeTemplateSQLDAO) Get(ctx context.Context, tx *db.Tx, id uuid.UUID) (*IpxeTemplate, error) {
	ctx, span := itd.tracerSpan.CreateChildInCurrentContext(ctx, "IpxeTemplateDAO.Get")
	if span != nil {
		defer span.End()
		itd.tracerSpan.SetAttribute(span, "id", id)
	}

	it := &IpxeTemplate{}

	err := db.GetIDB(tx, itd.dbSession).NewSelect().Model(it).Where("it.id = ?", id).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrDoesNotExist
		}
		return nil, err
	}

	return it, nil
}

// GetBySiteAndName returns an IpxeTemplate by site ID and template name
// Returns db.ErrDoesNotExist if the record is not found
func (itd IpxeTemplateSQLDAO) GetBySiteAndName(ctx context.Context, tx *db.Tx, siteID uuid.UUID, name string) (*IpxeTemplate, error) {
	ctx, span := itd.tracerSpan.CreateChildInCurrentContext(ctx, "IpxeTemplateDAO.GetBySiteAndName")
	if span != nil {
		defer span.End()
		itd.tracerSpan.SetAttribute(span, "site_id", siteID)
		itd.tracerSpan.SetAttribute(span, "name", name)
	}

	it := &IpxeTemplate{}

	err := db.GetIDB(tx, itd.dbSession).NewSelect().Model(it).
		Where("it.site_id = ?", siteID).
		Where("it.name = ?", name).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrDoesNotExist
		}
		return nil, err
	}

	return it, nil
}

// setQueryWithFilter populates the lookup query based on the specified filter
func (itd IpxeTemplateSQLDAO) setQueryWithFilter(filter IpxeTemplateFilterInput, query *bun.SelectQuery, span *stracer.CurrentContextSpan) (*bun.SelectQuery, error) {
	if len(filter.SiteIDs) > 0 {
		query = query.Where("it.site_id IN (?)", bun.In(filter.SiteIDs))
		if span != nil {
			itd.tracerSpan.SetAttribute(span, "site_ids", filter.SiteIDs)
		}
	}

	if len(filter.Names) > 0 {
		query = query.Where("it.name IN (?)", bun.In(filter.Names))
		if span != nil {
			itd.tracerSpan.SetAttribute(span, "names", filter.Names)
		}
	}

	if len(filter.Scopes) > 0 {
		query = query.Where("it.scope IN (?)", bun.In(filter.Scopes))
		if span != nil {
			itd.tracerSpan.SetAttribute(span, "scopes", filter.Scopes)
		}
	}

	return query, nil
}

// GetAll returns all IpxeTemplates with optional filters
// If orderBy is nil, records are ordered by IpxeTemplateOrderByDefault in ascending order
func (itd IpxeTemplateSQLDAO) GetAll(ctx context.Context, tx *db.Tx, filter IpxeTemplateFilterInput, page paginator.PageInput) ([]IpxeTemplate, int, error) {
	ctx, span := itd.tracerSpan.CreateChildInCurrentContext(ctx, "IpxeTemplateDAO.GetAll")
	if span != nil {
		defer span.End()
	}

	templates := []IpxeTemplate{}

	query := db.GetIDB(tx, itd.dbSession).NewSelect().Model(&templates)

	query, err := itd.setQueryWithFilter(filter, query, span)
	if err != nil {
		return templates, 0, err
	}

	if page.OrderBy == nil {
		page.OrderBy = paginator.NewDefaultOrderBy(IpxeTemplateOrderByDefault)
	}

	pager, err := paginator.NewPaginator(ctx, query, page.Offset, page.Limit, page.OrderBy, IpxeTemplateOrderByFields)
	if err != nil {
		return nil, 0, err
	}

	err = pager.Query.Limit(pager.Limit).Offset(pager.Offset).Scan(ctx)
	if err != nil {
		return nil, 0, err
	}

	return templates, pager.Total, nil
}

// Update updates specified fields of an existing IpxeTemplate
func (itd IpxeTemplateSQLDAO) Update(ctx context.Context, tx *db.Tx, input IpxeTemplateUpdateInput) (*IpxeTemplate, error) {
	ctx, span := itd.tracerSpan.CreateChildInCurrentContext(ctx, "IpxeTemplateDAO.Update")
	if span != nil {
		defer span.End()
		itd.tracerSpan.SetAttribute(span, "id", input.ID)
	}

	it := &IpxeTemplate{ID: input.ID}
	updatedFields := []string{"template", "required_params", "reserved_params", "required_artifacts", "scope", "updated"}

	it.Template = input.Template
	it.RequiredParams = input.RequiredParams
	it.ReservedParams = input.ReservedParams
	it.RequiredArtifacts = input.RequiredArtifacts
	it.Scope = input.Scope

	_, err := db.GetIDB(tx, itd.dbSession).NewUpdate().Model(it).Column(updatedFields...).Where("it.id = ?", input.ID).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return itd.Get(ctx, tx, it.ID)
}

// Delete removes an IpxeTemplate by ID
func (itd IpxeTemplateSQLDAO) Delete(ctx context.Context, tx *db.Tx, id uuid.UUID) error {
	ctx, span := itd.tracerSpan.CreateChildInCurrentContext(ctx, "IpxeTemplateDAO.Delete")
	if span != nil {
		defer span.End()
		itd.tracerSpan.SetAttribute(span, "id", id)
	}

	it := &IpxeTemplate{ID: id}

	_, err := db.GetIDB(tx, itd.dbSession).NewDelete().Model(it).Where("id = ?", id).Exec(ctx)
	return err
}

// NewIpxeTemplateDAO returns a new IpxeTemplateDAO
func NewIpxeTemplateDAO(dbSession *db.Session) IpxeTemplateDAO {
	return &IpxeTemplateSQLDAO{
		dbSession:  dbSession,
		tracerSpan: stracer.NewTracerSpan(),
	}
}
