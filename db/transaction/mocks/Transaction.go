package mocks

import "github.com/stretchr/testify/mock"

import "github.com/cloudwan/gohan/db/pagination"
import "github.com/cloudwan/gohan/schema"
import "github.com/jmoiron/sqlx"

// Transaction mock
type Transaction struct {
	mock.Mock
}

// Create mock
func (_m *Transaction) Create(_a0 *schema.Resource) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(*schema.Resource) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Update mock
func (_m *Transaction) Update(_a0 *schema.Resource) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(*schema.Resource) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// StateUpdate mock
func (_m *Transaction) StateUpdate(_a0 *schema.Resource) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(*schema.Resource) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Delete mock
func (_m *Transaction) Delete(_a0 *schema.Schema, _a1 interface{}) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*schema.Schema, interface{}) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Fetch mock
func (_m *Transaction) Fetch(_a0 *schema.Schema, _a1 interface{}, _a2 []string) (*schema.Resource, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 *schema.Resource
	if rf, ok := ret.Get(0).(func(*schema.Schema, interface{}, []string) *schema.Resource); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*schema.Resource)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*schema.Schema, interface{}, []string) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// List mock
func (_m *Transaction) List(_a0 *schema.Schema, _a1 map[string]interface{}, _a2 *pagination.Paginator) ([]*schema.Resource, uint64, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 []*schema.Resource
	if rf, ok := ret.Get(0).(func(*schema.Schema, map[string]interface{}, *pagination.Paginator) []*schema.Resource); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*schema.Resource)
		}
	}

	var r1 uint64
	if rf, ok := ret.Get(1).(func(*schema.Schema, map[string]interface{}, *pagination.Paginator) uint64); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Get(1).(uint64)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(*schema.Schema, map[string]interface{}, *pagination.Paginator) error); ok {
		r2 = rf(_a0, _a1, _a2)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// RawTransaction mock
func (_m *Transaction) RawTransaction() *sqlx.Tx {
	ret := _m.Called()

	var r0 *sqlx.Tx
	if rf, ok := ret.Get(0).(func() *sqlx.Tx); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*sqlx.Tx)
		}
	}

	return r0
}

// Query mock
func (_m *Transaction) Query(_a0 *schema.Schema, _a1 string, _a2 []interface{}) ([]*schema.Resource, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 []*schema.Resource
	if rf, ok := ret.Get(0).(func(*schema.Schema, string, []interface{}) []*schema.Resource); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*schema.Resource)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*schema.Schema, string, []interface{}) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Commit mock
func (_m *Transaction) Commit() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Close mock
func (_m *Transaction) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Closed mock
func (_m *Transaction) Closed() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}
