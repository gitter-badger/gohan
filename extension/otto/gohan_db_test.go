package otto_test

import (
	"errors"

	"github.com/cloudwan/gohan/db/transaction/mocks"
	"github.com/cloudwan/gohan/extension/otto"
	"github.com/cloudwan/gohan/schema"
	"github.com/cloudwan/gohan/server/middleware"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GohanDb", func() {

	Describe("gohan_db_sql_make_columns", func() {
		Context("when a valid schema ID is given", func() {
			It("returns column names in Gohan DB compatible format", func() {
				extension, err := schema.NewExtension(map[string]interface{}{
					"id": "test_extension",
					"code": `
					  gohan_register_handler("test_event", function(context){
					    context.resp = gohan_db_sql_make_columns("test");
					  });`,
					"path": ".*",
				})
				Expect(err).ToNot(HaveOccurred())
				extensions := []*schema.Extension{extension}
				env := otto.NewEnvironment(testDB, &middleware.FakeIdentity{})
				Expect(env.LoadExtensionsForPath(extensions, "test_path")).To(Succeed())

				context := map[string]interface{}{}
				Expect(env.HandleEvent("test_event", context)).To(Succeed())
				Expect(context["resp"]).To(ContainElement("tests.`id` as `tests__id`"))
				Expect(context["resp"]).To(ContainElement("tests.`tenant_id` as `tests__tenant_id`"))
				Expect(context["resp"]).To(ContainElement("tests.`test_string` as `tests__test_string`"))
			})
		})

		Context("when an invalid schema ID is given", func() {
			It("returns an error", func() {
				extension, err := schema.NewExtension(map[string]interface{}{
					"id": "test_extension",
					"code": `
					  gohan_register_handler("test_event", function(context){
					    context.resp = gohan_db_sql_make_columns("NOT EXIST");
					  });`,
					"path": ".*",
				})
				Expect(err).ToNot(HaveOccurred())
				extensions := []*schema.Extension{extension}
				env := otto.NewEnvironment(testDB, &middleware.FakeIdentity{})
				Expect(env.LoadExtensionsForPath(extensions, "test_path")).To(Succeed())

				context := map[string]interface{}{}
				err = env.HandleEvent("test_event", context)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(MatchRegexp("test_event: Unknown schema 'NOT EXIST'"))
			})
		})

	})

	Describe("gohan_db_query", func() {
		Context("when valid parameters are given", func() {
			It("returns resources in db", func() {
				extension, err := schema.NewExtension(map[string]interface{}{
					"id": "test_extension",
					"code": `
					  gohan_register_handler("test_event", function(context){
					    var tx = context.transaction;
					    context.resp = gohan_db_query(
					      tx,
					      "test",
					      "SELECT DUMMY",
					      ["tenant0", "obj1"]
					    );
					  });`,
					"path": ".*",
				})
				Expect(err).ToNot(HaveOccurred())
				extensions := []*schema.Extension{extension}
				env := otto.NewEnvironment(testDB, &middleware.FakeIdentity{})
				Expect(env.LoadExtensionsForPath(extensions, "test_path")).To(Succeed())

				manager := schema.GetManager()
				s, ok := manager.Schema("test")
				Expect(ok).To(BeTrue())

				fakeResources := []map[string]interface{}{
					map[string]interface{}{"tenant_id": "t0", "test_string": "str0"},
					map[string]interface{}{"tenant_id": "t1", "test_string": "str1"},
				}

				r0, err := schema.NewResource(s, fakeResources[0])
				Expect(err).ToNot(HaveOccurred())
				r1, err := schema.NewResource(s, fakeResources[1])
				Expect(err).ToNot(HaveOccurred())

				var fakeTx = new(mocks.Transaction)
				fakeTx.On(
					"Query", s, "SELECT DUMMY", []interface{}{"tenant0", "obj1"},
				).Return(
					[]*schema.Resource{r0, r1}, nil,
				)

				context := map[string]interface{}{
					"transaction": fakeTx,
				}
				Expect(env.HandleEvent("test_event", context)).To(Succeed())
				Expect(context["resp"]).To(Equal(fakeResources))
			})
		})

		Context("When an invalid transaction is provided", func() {
			It("fails and return an error", func() {
				extension, err := schema.NewExtension(map[string]interface{}{
					"id": "test_extension",
					"code": `
					  gohan_register_handler("test_event", function(context){
					    var tx = context.transaction;
					    context.resp = gohan_db_query(
					      tx,
					      "test",
					      "SELECT DUMMY",
					      ["tenant0", "obj1"]
					    );
					  });`,
					"path": ".*",
				})
				Expect(err).ToNot(HaveOccurred())
				extensions := []*schema.Extension{extension}
				env := otto.NewEnvironment(testDB, &middleware.FakeIdentity{})
				Expect(env.LoadExtensionsForPath(extensions, "test_path")).To(Succeed())

				context := map[string]interface{}{
					"transaction": "not_a_transaction",
				}

				err = env.HandleEvent("test_event", context)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(MatchRegexp("test_event: No transaction"))
			})
		})

		Context("When an invalid schema ID is provided", func() {
			It("fails and return an error", func() {
				extension, err := schema.NewExtension(map[string]interface{}{
					"id": "test_extension",
					"code": `
					  gohan_register_handler("test_event", function(context){
					    var tx = context.transaction;
					    context.resp = gohan_db_query(
					      tx,
					      "INVALID_SCHEMA_ID",
					      "SELECT DUMMY",
					      ["tenant0", "obj1"]
					    );
					  });`,
					"path": ".*",
				})
				Expect(err).ToNot(HaveOccurred())
				extensions := []*schema.Extension{extension}
				env := otto.NewEnvironment(testDB, &middleware.FakeIdentity{})
				Expect(env.LoadExtensionsForPath(extensions, "test_path")).To(Succeed())

				context := map[string]interface{}{
					"transaction": new(mocks.Transaction),
				}
				err = env.HandleEvent("test_event", context)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(MatchRegexp("test_event: Unknown schema 'INVALID_SCHEMA_ID'"))
			})
		})

		Context("When an invalid array is provided to arguments", func() {
			It("fails and return an error", func() {
				extension, err := schema.NewExtension(map[string]interface{}{
					"id": "test_extension",
					"code": `
					  gohan_register_handler("test_event", function(context){
					    var tx = context.transaction;
					    context.resp = gohan_db_query(
					      tx,
					      "test",
					      "SELECT DUMMY",
					      "THIS IS NOT AN ARRAY"
					    );
					  });`,
					"path": ".*",
				})
				Expect(err).ToNot(HaveOccurred())
				extensions := []*schema.Extension{extension}
				env := otto.NewEnvironment(testDB, &middleware.FakeIdentity{})
				Expect(env.LoadExtensionsForPath(extensions, "test_path")).To(Succeed())

				context := map[string]interface{}{
					"transaction": new(mocks.Transaction),
				}
				err = env.HandleEvent("test_event", context)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(MatchRegexp("test_event: Gievn arguments is not \\[\\]interface\\{\\}"))
			})
		})

		Context("When an error occured while processing the query", func() {
			It("fails and return an error", func() {
				extension, err := schema.NewExtension(map[string]interface{}{
					"id": "test_extension",
					"code": `
					  gohan_register_handler("test_event", function(context){
					    var tx = context.transaction;
					    context.resp = gohan_db_query(
					      tx,
					      "test",
					      "SELECT DUMMY",
					      []
					    );
					  });`,
					"path": ".*",
				})
				Expect(err).ToNot(HaveOccurred())
				extensions := []*schema.Extension{extension}
				env := otto.NewEnvironment(testDB, &middleware.FakeIdentity{})
				Expect(env.LoadExtensionsForPath(extensions, "test_path")).To(Succeed())

				manager := schema.GetManager()
				s, ok := manager.Schema("test")
				Expect(ok).To(BeTrue())

				var fakeTx = new(mocks.Transaction)
				fakeTx.On(
					"Query", s, "SELECT DUMMY", []interface{}{},
				).Return(
					nil, errors.New("SOMETHING HAPPENED"),
				)

				context := map[string]interface{}{
					"transaction": fakeTx,
				}
				err = env.HandleEvent("test_event", context)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(MatchRegexp("test_event: Error during gohan_db_query: SOMETHING HAPPEN"))
			})
		})

	})
})