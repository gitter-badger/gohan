// Copyright (C) 2015 NTT Innovation Institute, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resources

import (
	"fmt"
	"strings"

	"github.com/cloudwan/gohan/db"
	"github.com/cloudwan/gohan/db/pagination"
	"github.com/cloudwan/gohan/db/transaction"
	"github.com/cloudwan/gohan/extension"

	"github.com/cloudwan/gohan/schema"
	"github.com/cloudwan/gohan/server/middleware"
	"github.com/twinj/uuid"
)

//ResourceProblem describes the kind of problem that occurred during resource manipulation.
type ResourceProblem int

//The possible resource problems
const (
	InternalServerError ResourceProblem = iota
	WrongQuery
	WrongData
	NotFound
	DeleteFailed
	CreateFailed
	UpdateFailed
	hlsearch

	Unauthorized
)

// ResourceError is created when an anticipated problem has occured during resource manipulations.
// It contains the original error, a message to the user and an integer indicating the type of the problem.
type ResourceError struct {
	error
	Message string
	Problem ResourceProblem
}

//NewResourceError returns a new resource error
func NewResourceError(err error, message string, problem ResourceProblem) ResourceError {
	return ResourceError{err, message, problem}
}

// ExtensionError is created when a problem has occured during event handling. It contains the information
// required to reraise the javascript exception that caused this error.
type ExtensionError struct {
	error
	ExceptionInfo map[string]interface{}
}

//InTransaction executes function in the db transaction and set it to the context
func InTransaction(context middleware.Context, dataStore db.DB, f func() error) error {
	if context["transaction"] != nil {
		return fmt.Errorf("cannot create nested transaction")
	}
	aTransaction, err := dataStore.Begin()
	if err != nil {
		return fmt.Errorf("cannot create transaction: %v", err)
	}
	defer aTransaction.Close()
	context["transaction"] = aTransaction

	err = f()
	if err != nil {
		return err
	}

	err = aTransaction.Commit()
	if err != nil {
		return fmt.Errorf("commit error : %s", err)
	}
	delete(context, "transaction")
	return nil
}

// ApplyPolicyForResources applies policy filtering for response
func ApplyPolicyForResources(context middleware.Context, resourceSchema *schema.Schema) error {
	policy := context["policy"].(*schema.Policy)
	rawResponse, ok := context["response"]
	if !ok {
		return fmt.Errorf("No response")
	}
	response, ok := rawResponse.(map[string]interface{})
	if !ok {
		return fmt.Errorf("extension returned invalid JSON: %v", rawResponse)
	}
	resources, ok := response[resourceSchema.Plural].([]interface{})
	if !ok {
		return nil
	}
	data := []interface{}{}
	for _, resource := range resources {
		data = append(data, policy.Filter(resource.(map[string]interface{})))
	}
	response[resourceSchema.Plural] = data
	return nil
}

// ApplyPolicyForResource applies policy filtering for response
func ApplyPolicyForResource(context middleware.Context, resourceSchema *schema.Schema) error {
	policy := context["policy"].(*schema.Policy)
	rawResponse, ok := context["response"]
	if !ok {
		return fmt.Errorf("No response")
	}
	response, ok := rawResponse.(map[string]interface{})
	if !ok {
		return fmt.Errorf("extension returned invalid JSON: %v", rawResponse)
	}
	resource, ok := response[resourceSchema.Singular]
	if !ok {
		return nil
	}
	response[resourceSchema.Singular] = policy.Filter(resource.(map[string]interface{}))
	return nil
}

//GetResources returns specified resources without calling non in_transaction events
func GetResources(context middleware.Context, dataStore db.DB, resourceSchema *schema.Schema, filter map[string]interface{}, paginator *pagination.Paginator) error {
	return InTransaction(
		context, dataStore,
		func() error {
			return GetResourcesInTransaction(context, resourceSchema, filter, paginator)
		},
	)
}

//GetResourcesInTransaction returns specified resources without calling non in_transaction events
func GetResourcesInTransaction(context middleware.Context, resourceSchema *schema.Schema, filter map[string]interface{}, paginator *pagination.Paginator) error {
	mainTransaction := context["transaction"].(transaction.Transaction)
	auth := context["auth"].(schema.Authorization)
	response := map[string]interface{}{}

	environmentManager := extension.GetManager()
	environment, ok := environmentManager.GetEnvironment(resourceSchema.ID)
	if !ok {
		return fmt.Errorf("no environment for schema")
	}

	if err := handleEvent(context, environment, "pre_list_in_transaction"); err != nil {
		return err
	}
	var err error
	var total uint64
	list := []*schema.Resource{}
	if resourceSchema.ID == "schema" {
		manager := schema.GetManager()
		for _, currentSchema := range manager.OrderedSchemas() {
			trimmedSchema, err := GetSchema(currentSchema, auth)
			if err != nil {
				return err
			}
			if trimmedSchema != nil {
				list = append(list, trimmedSchema)
				total = total + 1
			}
		}
	} else {
		list, total, err = mainTransaction.List(resourceSchema, filter, paginator)
		if err != nil {
			response[resourceSchema.Plural] = []interface{}{}
			context["response"] = response
			return err
		}
	}

	data := []interface{}{}
	for _, resource := range list {
		data = append(data, resource.Data())
	}
	response[resourceSchema.Plural] = data

	context["response"] = response
	context["total"] = total

	if err := handleEvent(context, environment, "post_list_in_transaction"); err != nil {
		return err
	}
	return nil
}

//FilterFromQueryParameter makes list filter from query
func FilterFromQueryParameter(resourceSchema *schema.Schema,
	queryParameters map[string][]string) map[string]interface{} {
	filter := map[string]interface{}{}
	for key, value := range queryParameters {
		if _, err := resourceSchema.GetPropertyByID(key); err != nil {
			log.Info("Resource %s does not have %s property, ignoring filter.")
			continue
		}
		filter[key] = value
	}
	return filter
}

// GetMultipleResources returns all resources specified by the schema and query parameters
func GetMultipleResources(context middleware.Context, dataStore db.DB, resourceSchema *schema.Schema, queryParameters map[string][]string) error {
	log.Debug("Start get multiple resources!!")
	auth := context["auth"].(schema.Authorization)
	policy, err := loadPolicy(context, "read", resourceSchema.GetPluralURL(), auth)
	if err != nil {
		return err
	}

	filter := FilterFromQueryParameter(resourceSchema, queryParameters)

	if policy.RequireOwner() {
		filter["tenant_id"] = policy.GetTenantIDFilter(schema.ActionRead, auth.TenantID())
	}
	filter = policy.Filter(filter)

	paginator, err := pagination.FromURLQuery(resourceSchema, queryParameters)
	if err != nil {
		return ResourceError{err, err.Error(), WrongQuery}
	}

	environmentManager := extension.GetManager()
	environment, ok := environmentManager.GetEnvironment(resourceSchema.ID)
	if !ok {
		return fmt.Errorf("No environment for schema")
	}
	if err := handleEvent(context, environment, "pre_list"); err != nil {
		return err
	}
	if rawResponse, ok := context["response"]; ok {
		if _, ok := rawResponse.(map[string]interface{}); ok {
			return nil
		}
		return fmt.Errorf("extension returned invalid JSON: %v", rawResponse)
	}

	if err := GetResources(context, dataStore, resourceSchema, filter, paginator); err != nil {
		return err
	}

	if err := handleEvent(context, environment, "post_list"); err != nil {
		return err
	}

	if err := ApplyPolicyForResources(context, resourceSchema); err != nil {
		return err
	}

	return nil
}

// GetSingleResource returns the resource specified by the schema and ID
func GetSingleResource(context middleware.Context, dataStore db.DB, resourceSchema *schema.Schema, resourceID string) error {
	context["id"] = resourceID
	auth := context["auth"].(schema.Authorization)
	policy, err := loadPolicy(context, "read", strings.Replace(resourceSchema.GetSingleURL(), ":id", resourceID, 1), auth)
	if err != nil {
		return err
	}
	context["policy"] = policy

	environmentManager := extension.GetManager()
	environment, ok := environmentManager.GetEnvironment(resourceSchema.ID)
	if !ok {
		return fmt.Errorf("No environment for schema")
	}
	if err := handleEvent(context, environment, "pre_show"); err != nil {
		return err
	}
	if rawResponse, ok := context["response"]; ok {
		if _, ok := rawResponse.(map[string]interface{}); ok {
			return nil
		}
		return fmt.Errorf("extension returned invalid JSON: %v", rawResponse)
	}

	if err := InTransaction(
		context, dataStore,
		func() error {
			return GetSingleResourceInTransaction(context, resourceSchema, resourceID, policy.GetTenantIDFilter(schema.ActionRead, auth.TenantID()))
		},
	); err != nil {
		return err
	}

	if err := handleEvent(context, environment, "post_show"); err != nil {
		return err
	}
	if err := ApplyPolicyForResource(context, resourceSchema); err != nil {
		return err
	}
	return nil
}

//GetSingleResourceInTransaction get resource in single transaction
func GetSingleResourceInTransaction(context middleware.Context, resourceSchema *schema.Schema, resourceID string, tenantIDs []string) (err error) {
	mainTransaction := context["transaction"].(transaction.Transaction)
	auth := context["auth"].(schema.Authorization)
	environmentManager := extension.GetManager()
	environment, ok := environmentManager.GetEnvironment(resourceSchema.ID)
	if !ok {
		return fmt.Errorf("no environment for schema")
	}

	if err := handleEvent(context, environment, "pre_show_in_transaction"); err != nil {
		return err
	}
	if rawResponse, ok := context["response"]; ok {
		if _, ok := rawResponse.(map[string]interface{}); ok {
			return nil
		}
		return fmt.Errorf("extension returned invalid JSON: %v", rawResponse)
	}
	var object *schema.Resource
	if resourceSchema.ID == "schema" {
		manager := schema.GetManager()
		requestedSchema, _ := manager.Schema(resourceID)
		object, err = GetSchema(requestedSchema, auth)
	} else {
		object, err = mainTransaction.Fetch(resourceSchema, resourceID, tenantIDs)
	}

	if err != nil || object == nil {
		return ResourceError{err, "", NotFound}
	}

	response := map[string]interface{}{}
	response[resourceSchema.Singular] = object.Data()
	context["response"] = response

	if err := handleEvent(context, environment, "post_show_in_transaction"); err != nil {
		return err
	}
	return
}

// CreateResource creates the resource specified by the schema and dataMap
func CreateResource(
	context middleware.Context,
	dataStore db.DB,
	identityService middleware.IdentityService,
	resourceSchema *schema.Schema,
	dataMap map[string]interface{},
) error {
	manager := schema.GetManager()
	// Load environment
	environmentManager := extension.GetManager()
	environment, ok := environmentManager.GetEnvironment(resourceSchema.ID)
	if !ok {
		return fmt.Errorf("No environment for schema")
	}
	auth := context["auth"].(schema.Authorization)

	//LoadPolicy
	policy, err := loadPolicy(context, "create", resourceSchema.GetPluralURL(), auth)
	if err != nil {
		return err
	}

	_, err = resourceSchema.GetPropertyByID("tenant_id")
	if _, ok := dataMap["tenant_id"]; err == nil && !ok {
		dataMap["tenant_id"] = context["tenant_id"]
	}

	if tenantID, ok := dataMap["tenant_id"]; ok {
		dataMap["tenant_name"], err = identityService.GetTenantName(tenantID.(string))
		if err != nil {
			return ResourceError{err, err.Error(), Unauthorized}
		}
	}

	//Apply policy for api input
	err = policy.Check(schema.ActionCreate, auth, dataMap)
	if err != nil {
		return ResourceError{err, err.Error(), Unauthorized}
	}
	delete(dataMap, "tenant_name")

	context["resource"] = dataMap

	if err := handleEvent(context, environment, "pre_create"); err != nil {
		return err
	}

	if resourceData, ok := context["resource"].(map[string]interface{}); ok {
		dataMap = resourceData
	}

	//Validation
	err = resourceSchema.ValidateOnCreate(dataMap)
	if err != nil {
		return ResourceError{err, fmt.Sprintf("Validation error: %s", err), WrongData}
	}

	if _, ok := dataMap["id"]; !ok {
		dataMap["id"] = uuid.NewV4().String()
	}
	resource, err := manager.LoadResource(resourceSchema.ID, dataMap)
	if err != nil {
		return err
	}

	//Fillup default
	err = resource.PopulateDefaults()
	if err != nil {
		return err
	}

	context["resource"] = resource.Data()

	if err := InTransaction(
		context, dataStore,
		func() error {
			return CreateResourceInTransaction(context, resource)
		},
	); err != nil {
		return err
	}

	if err := handleEvent(context, environment, "post_create"); err != nil {
		return err
	}

	if err := ApplyPolicyForResource(context, resourceSchema); err != nil {
		return err
	}
	return nil
}

//CreateResourceInTransaction craete db resource model in transaction
func CreateResourceInTransaction(context middleware.Context, resource *schema.Resource) error {
	resourceSchema := resource.Schema()
	mainTransaction := context["transaction"].(transaction.Transaction)
	environmentManager := extension.GetManager()
	environment, ok := environmentManager.GetEnvironment(resourceSchema.ID)
	if !ok {
		return fmt.Errorf("No environment for schema")
	}
	if err := handleEvent(context, environment, "pre_create_in_transaction"); err != nil {
		return err
	}
	if err := mainTransaction.Create(resource); err != nil {
		log.Debug("%s transaction error", err)
		return ResourceError{
			err,
			fmt.Sprintf("Failed to store data in database: %v", err),
			CreateFailed}
	}

	response := map[string]interface{}{}
	response[resourceSchema.Singular] = resource.Data()
	context["response"] = response

	if err := handleEvent(context, environment, "post_create_in_transaction"); err != nil {
		return err
	}

	return nil
}

// UpdateResource updates the resource specified by the schema and ID using the dataMap
func UpdateResource(
	context middleware.Context,
	dataStore db.DB, identityService middleware.IdentityService,
	resourceSchema *schema.Schema,
	resourceID string, dataMap map[string]interface{},
) error {

	context["id"] = resourceID

	//load environment
	environmentManager := extension.GetManager()
	environment, ok := environmentManager.GetEnvironment(resourceSchema.ID)
	if !ok {
		return fmt.Errorf("No environment for schema")
	}

	auth := context["auth"].(schema.Authorization)

	//load policy
	policy, err := loadPolicy(context, "update", strings.Replace(resourceSchema.GetSingleURL(), ":id", resourceID, 1), auth)
	if err != nil {
		return err
	}

	//fillup default values
	if tenantID, ok := dataMap["tenant_id"]; ok {
		dataMap["tenant_name"], err = identityService.GetTenantName(tenantID.(string))
	}
	if err != nil {
		return ResourceError{err, err.Error(), Unauthorized}
	}

	//check policy
	err = policy.Check(schema.ActionUpdate, auth, dataMap)
	delete(dataMap, "tenant_name")
	if err != nil {
		return ResourceError{err, err.Error(), Unauthorized}
	}
	context["resource"] = dataMap

	if err := handleEvent(context, environment, "pre_update"); err != nil {
		return err
	}

	if resourceData, ok := context["resource"].(map[string]interface{}); ok {
		dataMap = resourceData
	}

	if err := InTransaction(
		context, dataStore,
		func() error {
			return UpdateResourceInTransaction(context, resourceSchema, resourceID, dataMap, policy.GetTenantIDFilter(schema.ActionUpdate, auth.TenantID()))
		},
	); err != nil {
		return err
	}

	if err := handleEvent(context, environment, "post_update"); err != nil {
		return err
	}

	if err := ApplyPolicyForResource(context, resourceSchema); err != nil {
		return err
	}
	return nil
}

// UpdateResourceInTransaction updates resource in db in transaction
func UpdateResourceInTransaction(
	context middleware.Context,
	resourceSchema *schema.Schema, resourceID string,
	dataMap map[string]interface{}, tenantIDs []string) error {

	manager := schema.GetManager()
	mainTransaction := context["transaction"].(transaction.Transaction)
	environmentManager := extension.GetManager()
	environment, ok := environmentManager.GetEnvironment(resourceSchema.ID)
	if !ok {
		return fmt.Errorf("No environment for schema")
	}
	resource, err := mainTransaction.Fetch(
		resourceSchema, resourceID, tenantIDs)
	if err != nil {
		return ResourceError{err, err.Error(), WrongQuery}
	}
	err = resource.Update(dataMap)
	if err != nil {
		return ResourceError{err, err.Error(), WrongData}
	}
	context["resource"] = resource.Data()

	if err := handleEvent(context, environment, "pre_update_in_transaction"); err != nil {
		return err
	}

	dataMap, ok = context["resource"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("Resource not JSON: %s", err)
	}
	resource, err = manager.LoadResource(resourceSchema.ID, dataMap)
	if err != nil {
		return fmt.Errorf("Loading Resource failed: %s", err)
	}

	err = mainTransaction.Update(resource)
	if err != nil {
		return ResourceError{err, fmt.Sprintf("Failed to store data in database: %v", err), UpdateFailed}
	}
	resourceSchema.HandleUpdate(resource)

	response := map[string]interface{}{}
	response[resourceSchema.Singular] = resource.Data()
	context["response"] = response

	if err := handleEvent(context, environment, "post_update_in_transaction"); err != nil {
		return err
	}

	return nil
}

// DeleteResource deletes the resource specified by the schema and ID
func DeleteResource(context middleware.Context,
	dataStore db.DB,
	resourceSchema *schema.Schema,
	resourceID string,
) error {
	context["id"] = resourceID
	environmentManager := extension.GetManager()
	environment, ok := environmentManager.GetEnvironment(resourceSchema.ID)
	if !ok {
		return fmt.Errorf("No environment for schema")
	}
	auth := context["auth"].(schema.Authorization)
	policy, err := loadPolicy(context, "delete", strings.Replace(resourceSchema.GetSingleURL(), ":id", resourceID, 1), auth)
	if err != nil {
		return err
	}

	preTransaction, err := dataStore.Begin()
	if err != nil {
		return fmt.Errorf("cannot create transaction: %v", err)
	}
	resource, fetchErr := preTransaction.Fetch(resourceSchema, resourceID, policy.GetTenantIDFilter(schema.ActionDelete, auth.TenantID()))
	preTransaction.Close()
	context["resource"] = resource

	if err := handleEvent(context, environment, "pre_delete"); err != nil {
		return err
	}

	if fetchErr != nil {
		return ResourceError{err, "", NotFound}
	}

	if err := InTransaction(
		context, dataStore,
		func() error {
			return DeleteResourceInTransaction(context, resourceSchema, resourceID)
		},
	); err != nil {
		return err
	}

	if err := handleEvent(context, environment, "post_delete"); err != nil {
		return err
	}
	return nil
}

//DeleteResourceInTransaction deletes resources in a transaction
func DeleteResourceInTransaction(context middleware.Context, resourceSchema *schema.Schema, resourceID string) error {
	mainTransaction := context["transaction"].(transaction.Transaction)
	environmentManager := extension.GetManager()
	environment, ok := environmentManager.GetEnvironment(resourceSchema.ID)
	if !ok {
		return fmt.Errorf("No environment for schema")
	}
	if err := handleEvent(context, environment, "pre_delete_in_transaction"); err != nil {
		return err
	}

	err := mainTransaction.Delete(resourceSchema, resourceID)
	if err != nil {
		return ResourceError{err, "", DeleteFailed}
	}

	if err := handleEvent(context, environment, "post_delete_in_transaction"); err != nil {
		return err
	}
	return nil
}

// ActionResource runs custom action on resource
func ActionResource(context middleware.Context, dataStore db.DB, identityService middleware.IdentityService,
	resourceSchema *schema.Schema, action schema.Action, resourceID string, data interface{},
) error {
	actionSchema := action.InputSchema
	context["input"] = data
	context["id"] = resourceID

	environmentManager := extension.GetManager()
	environment, ok := environmentManager.GetEnvironment(resourceSchema.ID)
	if !ok {
		return fmt.Errorf("No environment for schema")
	}

	err := resourceSchema.Validate(actionSchema, data)
	if err != nil {
		return ResourceError{err, fmt.Sprintf("Validation error: %s", err), WrongData}
	}

	if err := handleEvent(context, environment, fmt.Sprintf("pre_%s", action.ID)); err != nil {
		return err
	}

	if err := InTransaction(context, dataStore, func() error {
		return handleEvent(context, environment, fmt.Sprintf("pre_%s_in_transaction", action.ID))
	}); err != nil {
		return err
	}

	if err := handleEvent(context, environment, action.ID); err != nil {
		return err
	}

	if err := InTransaction(context, dataStore, func() error {
		return handleEvent(context, environment, fmt.Sprintf("post_%s_in_transaction", action.ID))
	}); err != nil {
		return err
	}

	if err := handleEvent(context, environment, fmt.Sprintf("post_%s", action.ID)); err != nil {
		return err
	}

	if rawResponse, ok := context["response"]; ok {
		if _, ok := rawResponse.(map[string]interface{}); ok {
			return nil
		}
		return fmt.Errorf("extension returned invalid JSON: %v", rawResponse)
	}

	return fmt.Errorf("no response")
}

func handleEvent(context middleware.Context, environment extension.Environment, event string) error {
	if err := environment.HandleEvent(event, context); err != nil {
		return fmt.Errorf("extension error: %s", err)
	}
	exceptionInfoRaw, ok := context["exception"]
	if !ok {
		return nil
	}
	exceptionInfo, ok := exceptionInfoRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("extension returned invalid error information")
	}
	exceptionMessage := context["exception_message"]
	return ExtensionError{fmt.Errorf("%v", exceptionMessage), exceptionInfo}
}

func loadPolicy(context middleware.Context, action, path string, auth schema.Authorization) (*schema.Policy, error) {
	manager := schema.GetManager()
	policy, role := manager.PolicyValidate(action, path, auth)
	if policy == nil {
		err := fmt.Errorf(fmt.Sprintf("No matching policy: %s %s", action, path))
		return nil, ResourceError{err, err.Error(), Unauthorized}
	}
	context["policy"] = policy
	context["role"] = role
	return policy, nil
}

//GetSchema returns the schema filtered and trimmed for a specific user or nil when the user shouldn't see it at all
func GetSchema(s *schema.Schema, authorization schema.Authorization) (result *schema.Resource, err error) {
	manager := schema.GetManager()
	metaschema, _ := manager.Schema("schema")
	policy, _ := manager.PolicyValidate("read", s.GetPluralURL(), authorization)
	if policy == nil {
		return
	}
	originalRawSchema := s.RawData.(map[string]interface{})
	rawSchema := map[string]interface{}{}
	for key, value := range originalRawSchema {
		rawSchema[key] = value
	}
	originalSchema := originalRawSchema["schema"].(map[string]interface{})
	schemaSchema := map[string]interface{}{}
	for key, value := range originalSchema {
		schemaSchema[key] = value
	}
	rawSchema["schema"] = schemaSchema
	originalProperties := originalSchema["properties"].(map[string]interface{})
	schemaProperties := map[string]interface{}{}
	for key, value := range originalProperties {
		schemaProperties[key] = value
	}
	var schemaPropertiesOrder []interface{}
	if _, ok := originalSchema["propertiesOrder"]; ok {
		originalPropertiesOrder := originalSchema["propertiesOrder"].([]interface{})
		for _, value := range originalPropertiesOrder {
			schemaPropertiesOrder = append(schemaPropertiesOrder, value)
		}
	}
	var schemaRequired []interface{}
	if _, ok := originalSchema["required"]; ok {
		originalRequired := originalSchema["required"].([]interface{})
		for _, value := range originalRequired {
			schemaRequired = append(schemaRequired, value)
		}
	}
	schemaProperties, schemaPropertiesOrder, schemaRequired = policy.MetaFilter(schemaProperties, schemaPropertiesOrder, schemaRequired)
	schemaSchema["properties"] = schemaProperties
	schemaSchema["propertiesOrder"] = schemaPropertiesOrder
	schemaSchema["required"] = schemaRequired
	result, err = schema.NewResource(metaschema, rawSchema)
	if err != nil {
		log.Warning("%s %s", result, err)
		return
	}
	return
}
