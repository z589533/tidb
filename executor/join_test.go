// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package executor_test

import (
	"fmt"
	"time"

	. "github.com/pingcap/check"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/executor"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/plan"
	"github.com/pingcap/tidb/util/mock"
	"github.com/pingcap/tidb/util/testkit"
	"github.com/pingcap/tidb/util/testleak"
	"github.com/pingcap/tidb/util/types"
)

func (s *testSuite) TestNestedLoopJoin(c *C) {
	bigExec := &MockExec{Rows: []*executor.Row{
		{Data: types.MakeDatums(1)},
		{Data: types.MakeDatums(2)},
		{Data: types.MakeDatums(3)},
		{Data: types.MakeDatums(4)},
		{Data: types.MakeDatums(5)},
		{Data: types.MakeDatums(6)},
	}}
	smallExec := &MockExec{Rows: []*executor.Row{
		{Data: types.MakeDatums(1)},
		{Data: types.MakeDatums(2)},
		{Data: types.MakeDatums(3)},
		{Data: types.MakeDatums(4)},
		{Data: types.MakeDatums(5)},
		{Data: types.MakeDatums(6)},
	}}
	col0 := &expression.Column{Index: 0, RetType: types.NewFieldType(mysql.TypeLong)}
	col1 := &expression.Column{Index: 1, RetType: types.NewFieldType(mysql.TypeLong)}
	con := &expression.Constant{Value: types.NewDatum(6), RetType: types.NewFieldType(mysql.TypeLong)}
	bigFilter, _ := expression.NewFunction(mock.NewContext(), ast.LT, types.NewFieldType(mysql.TypeTiny), col0, con)
	smallFilter := bigFilter.Clone()
	otherFilter, _ := expression.NewFunction(mock.NewContext(), ast.EQ, types.NewFieldType(mysql.TypeTiny), col0, col1)
	join := &executor.NestedLoopJoinExec{
		BigExec:     bigExec,
		SmallExec:   smallExec,
		Ctx:         mock.NewContext(),
		BigFilter:   bigFilter,
		SmallFilter: smallFilter,
		OtherFilter: otherFilter,
	}
	row, err := join.Next()
	c.Check(err, IsNil)
	c.Check(row, NotNil)
	c.Check(fmt.Sprintf("%v %v", row.Data[0].GetValue(), row.Data[1].GetValue()), Equals, "1 1")
	row, err = join.Next()
	c.Check(err, IsNil)
	c.Check(row, NotNil)
	c.Check(fmt.Sprintf("%v %v", row.Data[0].GetValue(), row.Data[1].GetValue()), Equals, "2 2")
	row, err = join.Next()
	c.Check(err, IsNil)
	c.Check(row, NotNil)
	c.Check(fmt.Sprintf("%v %v", row.Data[0].GetValue(), row.Data[1].GetValue()), Equals, "3 3")
	row, err = join.Next()
	c.Check(err, IsNil)
	c.Check(row, NotNil)
	c.Check(fmt.Sprintf("%v %v", row.Data[0].GetValue(), row.Data[1].GetValue()), Equals, "4 4")
	row, err = join.Next()
	c.Check(err, IsNil)
	c.Check(row, NotNil)
	c.Check(fmt.Sprintf("%v %v", row.Data[0].GetValue(), row.Data[1].GetValue()), Equals, "5 5")
	row, err = join.Next()
	c.Check(err, IsNil)
	c.Check(row, IsNil)
}

func (s *testSuite) TestJoinPanic(c *C) {
	defer func() {
		s.cleanEnv(c)
		testleak.AfterTest(c)()
	}()
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists events")
	tk.MustExec("create table events (clock int, source int)")
	tk.MustQuery("SELECT * FROM events e JOIN (SELECT MAX(clock) AS clock FROM events e2 GROUP BY e2.source) e3 ON e3.clock=e.clock")
}

func (s *testSuite) TestJoin(c *C) {
	defer func() {
		s.cleanEnv(c)
		testleak.AfterTest(c)()
	}()
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t (c int)")
	tk.MustExec("insert t values (1)")
	cases := []struct {
		sql    string
		result [][]interface{}
	}{
		{
			"select 1 from t as a left join t as b on 0",
			testkit.Rows("1"),
		},
		{
			"select 1 from t as a join t as b on 1",
			testkit.Rows("1"),
		},
	}
	for _, ca := range cases {
		result := tk.MustQuery(ca.sql)
		result.Check(ca.result)
	}

	tk.MustExec("drop table if exists t")
	tk.MustExec("drop table if exists t1")
	tk.MustExec("create table t(c1 int, c2 int)")
	tk.MustExec("create table t1(c1 int, c2 int)")
	tk.MustExec("insert into t values(1,1),(2,2)")
	tk.MustExec("insert into t1 values(2,3),(4,4)")
	result := tk.MustQuery("select * from t left outer join t1 on t.c1 = t1.c1 where t.c1 = 1 or t1.c2 > 20")
	result.Check(testkit.Rows("1 1 <nil> <nil>"))
	result = tk.MustQuery("select * from t1 right outer join t on t.c1 = t1.c1 where t.c1 = 1 or t1.c2 > 20")
	result.Check(testkit.Rows("<nil> <nil> 1 1"))
	result = tk.MustQuery("select * from t right outer join t1 on t.c1 = t1.c1 where t.c1 = 1 or t1.c2 > 20")
	result.Check(testkit.Rows())
	result = tk.MustQuery("select * from t left outer join t1 on t.c1 = t1.c1 where t1.c1 = 3 or false")
	result.Check(testkit.Rows())
	result = tk.MustQuery("select * from t left outer join t1 on t.c1 = t1.c1 and t.c1 != 1 order by t1.c1")
	result.Check(testkit.Rows("1 1 <nil> <nil>", "2 2 2 3"))

	tk.MustExec("drop table if exists t1")
	tk.MustExec("drop table if exists t2")
	tk.MustExec("drop table if exists t3")

	tk.MustExec("create table t1 (c1 int, c2 int)")
	tk.MustExec("create table t2 (c1 int, c2 int)")
	tk.MustExec("create table t3 (c1 int, c2 int)")

	tk.MustExec("insert into t1 values (1,1), (2,2), (3,3)")
	tk.MustExec("insert into t2 values (1,1), (3,3), (5,5)")
	tk.MustExec("insert into t3 values (1,1), (5,5), (9,9)")

	result = tk.MustQuery("select * from t1 left join t2 on t1.c1 = t2.c1 right join t3 on t2.c1 = t3.c1 order by t1.c1, t1.c2, t2.c1, t2.c2, t3.c1, t3.c2;")
	result.Check(testkit.Rows("<nil> <nil> <nil> <nil> 5 5", "<nil> <nil> <nil> <nil> 9 9", "1 1 1 1 1 1"))

	tk.MustExec("drop table if exists t1")
	tk.MustExec("create table t1 (c1 int)")
	tk.MustExec("insert into t1 values (1), (1), (1)")
	result = tk.MustQuery("select * from t1 a join t1 b on a.c1 = b.c1;")
	result.Check(testkit.Rows("1 1", "1 1", "1 1", "1 1", "1 1", "1 1", "1 1", "1 1", "1 1"))

	tk.MustExec("drop table if exists t")
	tk.MustExec("drop table if exists t1")
	tk.MustExec("create table t(c1 int,c2 double)")
	tk.MustExec("create table t1(c1 double,c2 int)")
	tk.MustExec("insert into t values (1, 2), (1, NULL)")
	tk.MustExec("insert into t1 values (1, 2), (1, NULL)")
	result = tk.MustQuery("select * from t a , t1 b where (a.c1, a.c2) = (b.c1, b.c2);")
	result.Check(testkit.Rows("1 2 1 2"))
	tk.MustExec("drop table if exists t")
	tk.MustExec("drop table if exists t1")
	tk.MustExec("create table t(c1 int, index k(c1))")
	tk.MustExec("create table t1(c1 int)")
	tk.MustExec("insert into t values (1),(2),(3),(4),(5),(6),(7)")
	tk.MustExec("insert into t1 values (1),(2),(3),(4),(5),(6),(7)")
	result = tk.MustQuery("select a.c1 from t a , t1 b where a.c1 = b.c1 order by a.c1;")
	result.Check(testkit.Rows("1", "2", "3", "4", "5", "6", "7"))
	result = tk.MustQuery("select a.c1 from t a , (select * from t1 limit 3) b where a.c1 = b.c1 order by b.c1;")
	result.Check(testkit.Rows("1", "2", "3"))

	plan.AllowCartesianProduct = false
	_, err := tk.Exec("select * from t, t1")
	c.Check(plan.ErrCartesianProductUnsupported.Equal(err), IsTrue)
	_, err = tk.Exec("select * from t left join t1 on 1")
	c.Check(plan.ErrCartesianProductUnsupported.Equal(err), IsTrue)
	_, err = tk.Exec("select * from t right join t1 on 1")
	c.Check(plan.ErrCartesianProductUnsupported.Equal(err), IsTrue)
	plan.AllowCartesianProduct = true
	tk.MustExec("drop table if exists t")
	tk.MustExec("drop table if exists t1")
	tk.MustExec("create table t(c1 int)")
	tk.MustExec("create table t1(c1 int unsigned)")
	tk.MustExec("insert into t values (1)")
	tk.MustExec("insert into t1 values (1)")
	result = tk.MustQuery("select t.c1 from t , t1 where t.c1 = t1.c1")
	result.Check(testkit.Rows("1"))
}

func (s *testSuite) TestMultiJoin(c *C) {
	defer func() {
		s.cleanEnv(c)
		testleak.AfterTest(c)()
	}()
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("create table t35(a35 int primary key, b35 int, x35 int)")
	tk.MustExec("create table t40(a40 int primary key, b40 int, x40 int)")
	tk.MustExec("create table t14(a14 int primary key, b14 int, x14 int)")
	tk.MustExec("create table t42(a42 int primary key, b42 int, x42 int)")
	tk.MustExec("create table t15(a15 int primary key, b15 int, x15 int)")
	tk.MustExec("create table t7(a7 int primary key, b7 int, x7 int)")
	tk.MustExec("create table t64(a64 int primary key, b64 int, x64 int)")
	tk.MustExec("create table t19(a19 int primary key, b19 int, x19 int)")
	tk.MustExec("create table t9(a9 int primary key, b9 int, x9 int)")
	tk.MustExec("create table t8(a8 int primary key, b8 int, x8 int)")
	tk.MustExec("create table t57(a57 int primary key, b57 int, x57 int)")
	tk.MustExec("create table t37(a37 int primary key, b37 int, x37 int)")
	tk.MustExec("create table t44(a44 int primary key, b44 int, x44 int)")
	tk.MustExec("create table t38(a38 int primary key, b38 int, x38 int)")
	tk.MustExec("create table t18(a18 int primary key, b18 int, x18 int)")
	tk.MustExec("create table t62(a62 int primary key, b62 int, x62 int)")
	tk.MustExec("create table t4(a4 int primary key, b4 int, x4 int)")
	tk.MustExec("create table t48(a48 int primary key, b48 int, x48 int)")
	tk.MustExec("create table t31(a31 int primary key, b31 int, x31 int)")
	tk.MustExec("create table t16(a16 int primary key, b16 int, x16 int)")
	tk.MustExec("create table t12(a12 int primary key, b12 int, x12 int)")
	tk.MustExec("insert into t35 values(1,1,1)")
	tk.MustExec("insert into t40 values(1,1,1)")
	tk.MustExec("insert into t14 values(1,1,1)")
	tk.MustExec("insert into t42 values(1,1,1)")
	tk.MustExec("insert into t15 values(1,1,1)")
	tk.MustExec("insert into t7 values(1,1,1)")
	tk.MustExec("insert into t64 values(1,1,1)")
	tk.MustExec("insert into t19 values(1,1,1)")
	tk.MustExec("insert into t9 values(1,1,1)")
	tk.MustExec("insert into t8 values(1,1,1)")
	tk.MustExec("insert into t57 values(1,1,1)")
	tk.MustExec("insert into t37 values(1,1,1)")
	tk.MustExec("insert into t44 values(1,1,1)")
	tk.MustExec("insert into t38 values(1,1,1)")
	tk.MustExec("insert into t18 values(1,1,1)")
	tk.MustExec("insert into t62 values(1,1,1)")
	tk.MustExec("insert into t4 values(1,1,1)")
	tk.MustExec("insert into t48 values(1,1,1)")
	tk.MustExec("insert into t31 values(1,1,1)")
	tk.MustExec("insert into t16 values(1,1,1)")
	tk.MustExec("insert into t12 values(1,1,1)")
	tk.MustExec("insert into t35 values(7,7,7)")
	tk.MustExec("insert into t40 values(7,7,7)")
	tk.MustExec("insert into t14 values(7,7,7)")
	tk.MustExec("insert into t42 values(7,7,7)")
	tk.MustExec("insert into t15 values(7,7,7)")
	tk.MustExec("insert into t7 values(7,7,7)")
	tk.MustExec("insert into t64 values(7,7,7)")
	tk.MustExec("insert into t19 values(7,7,7)")
	tk.MustExec("insert into t9 values(7,7,7)")
	tk.MustExec("insert into t8 values(7,7,7)")
	tk.MustExec("insert into t57 values(7,7,7)")
	tk.MustExec("insert into t37 values(7,7,7)")
	tk.MustExec("insert into t44 values(7,7,7)")
	tk.MustExec("insert into t38 values(7,7,7)")
	tk.MustExec("insert into t18 values(7,7,7)")
	tk.MustExec("insert into t62 values(7,7,7)")
	tk.MustExec("insert into t4 values(7,7,7)")
	tk.MustExec("insert into t48 values(7,7,7)")
	tk.MustExec("insert into t31 values(7,7,7)")
	tk.MustExec("insert into t16 values(7,7,7)")
	tk.MustExec("insert into t12 values(7,7,7)")
	result := tk.MustQuery(`SELECT x4,x8,x38,x44,x31,x9,x57,x48,x19,x40,x14,x12,x7,x64,x37,x18,x62,x35,x42,x15,x16 FROM
t35,t40,t14,t42,t15,t7,t64,t19,t9,t8,t57,t37,t44,t38,t18,t62,t4,t48,t31,t16,t12
WHERE b48=a57
AND a4=b19
AND a14=b16
AND b37=a48
AND a40=b42
AND a31=7
AND a15=b40
AND a38=b8
AND b15=a31
AND b64=a18
AND b12=a44
AND b7=a8
AND b35=a16
AND a12=b14
AND a64=b57
AND b62=a7
AND a35=b38
AND b9=a19
AND a62=b18
AND b4=a37
AND b44=a42`)
	result.Check(testkit.Rows("7 7 7 7 7 7 7 7 7 7 7 7 7 7 7 7 7 7 7 7 7"))
}

func (s *testSuite) TestSubquerySameTable(c *C) {
	defer func() {
		s.cleanEnv(c)
		testleak.AfterTest(c)()
	}()
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t (a int)")
	tk.MustExec("insert t values (1), (2)")
	result := tk.MustQuery("select a from t where exists(select 1 from t as x where x.a < t.a)")
	result.Check(testkit.Rows("2"))
	result = tk.MustQuery("select a from t where not exists(select 1 from t as x where x.a < t.a)")
	result.Check(testkit.Rows("1"))
}

func (s *testSuite) TestSubquery(c *C) {
	plan.JoinConcurrency = 1
	defer func() {
		s.cleanEnv(c)
		testleak.AfterTest(c)()
		plan.JoinConcurrency = 5
	}()
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t (c int, d int)")
	tk.MustExec("insert t values (1, 1)")
	tk.MustExec("insert t values (2, 2)")
	tk.MustExec("insert t values (3, 4)")
	tk.MustExec("commit")
	result := tk.MustQuery("select * from t where exists(select * from t k where t.c = k.c having sum(c) = 1)")
	result.Check(testkit.Rows("1 1"))
	result = tk.MustQuery("select * from t where exists(select k.c, k.d from t k, t p where t.c = k.d)")
	result.Check(testkit.Rows("1 1", "2 2"))
	result = tk.MustQuery("select 1 = (select count(*) from t where t.c = k.d) from t k")
	result.Check(testkit.Rows("1", "1", "0"))
	result = tk.MustQuery("select 1 = (select count(*) from t where exists( select * from t m where t.c = k.d)) from t k")
	result.Check(testkit.Rows("1", "1", "0"))
	result = tk.MustQuery("select t.c = any (select count(*) from t) from t")
	result.Check(testkit.Rows("0", "0", "1"))
	result = tk.MustQuery("select * from t where (t.c, 6) = any (select count(*), sum(t.c) from t)")
	result.Check(testkit.Rows("3 4"))
	result = tk.MustQuery("select t.c from t where (t.c) < all (select count(*) from t)")
	result.Check(testkit.Rows("1", "2"))
	result = tk.MustQuery("select t.c from t where (t.c, t.d) = any (select * from t)")
	result.Check(testkit.Rows("1", "2", "3"))
	result = tk.MustQuery("select t.c from t where (t.c, t.d) != all (select * from t)")
	result.Check(testkit.Rows())
	result = tk.MustQuery("select (select count(*) from t where t.c = k.d) from t k")
	result.Check(testkit.Rows("1", "1", "0"))
	result = tk.MustQuery("select t.c from t where (t.c, t.d) in (select * from t)")
	result.Check(testkit.Rows("1", "2", "3"))
	result = tk.MustQuery("select t.c from t where (t.c, t.d) not in (select * from t)")
	result.Check(testkit.Rows())
	// = all empty set is true
	result = tk.MustQuery("select t.c from t where (t.c, t.d) != all (select * from t where d > 1000)")
	result.Check(testkit.Rows("1", "2", "3"))
	result = tk.MustQuery("select t.c from t where (t.c) < any (select c from t where d > 1000)")
	result.Check(testkit.Rows())
	tk.MustExec("insert t values (NULL, NULL)")
	result = tk.MustQuery("select (t.c) < any (select c from t) from t")
	result.Check(testkit.Rows("1", "1", "<nil>", "<nil>"))
	result = tk.MustQuery("select (10) > all (select c from t) from t")
	result.Check(testkit.Rows("<nil>", "<nil>", "<nil>", "<nil>"))
	result = tk.MustQuery("select (c) > all (select c from t) from t")
	result.Check(testkit.Rows("0", "0", "0", "<nil>"))

	tk.MustExec("drop table if exists a")
	tk.MustExec("create table a (c int, d int)")
	tk.MustExec("insert a values (1, 2)")
	tk.MustExec("drop table if exists b")
	tk.MustExec("create table b (c int, d int)")
	tk.MustExec("insert b values (2, 1)")

	result = tk.MustQuery("select * from a b where c = (select d from b a where a.c = 2 and b.c = 1)")
	result.Check(testkit.Rows("1 2"))

	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t(c int)")
	tk.MustExec("insert t values(10), (8), (7), (9), (11)")
	result = tk.MustQuery("select * from t where 9 in (select c from t s where s.c < t.c limit 3)")
	result.Check(testkit.Rows("10"))

	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t(id int, v int)")
	tk.MustExec("insert into t values(1, 1), (2, 2), (3, 3)")
	result = tk.MustQuery("select * from t where v=(select min(t1.v) from t t1, t t2, t t3 where t1.id=t2.id and t2.id=t3.id and t1.id=t.id)")
	result.Check(testkit.Rows("1 1", "2 2", "3 3"))

	result = tk.MustQuery("select exists (select t.id from t where s.id < 2 and t.id = s.id) from t s")
	result.Check(testkit.Rows("1", "0", "0"))

	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t(c int)")
	result = tk.MustQuery("select exists(select count(*) from t)")
	result.Check(testkit.Rows("1"))
}

func (s *testSuite) TestInSubquery(c *C) {
	defer func() {
		s.cleanEnv(c)
		testleak.AfterTest(c)()
	}()
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t (a int, b int)")
	tk.MustExec("insert t values (1, 1), (2, 1)")
	result := tk.MustQuery("select m1.a from t as m1 where m1.a in (select m2.b from t as m2)")
	result.Check(testkit.Rows("1"))
	result = tk.MustQuery("select m1.a from t as m1 where (3, m1.b) not in (select * from t as m2)")
	result.Check(testkit.Rows("1", "2"))
	result = tk.MustQuery("select m1.a from t as m1 where m1.a in (select m2.b+? from t as m2)", 1)
	result.Check(testkit.Rows("2"))
	tk.MustExec(`prepare stmt1 from 'select m1.a from t as m1 where m1.a in (select m2.b+? from t as m2)'`)
	tk.MustExec("set @a = 1")
	result = tk.MustQuery(`execute stmt1 using @a;`)
	result.Check(testkit.Rows("2"))
	tk.MustExec("set @a = 0")
	result = tk.MustQuery(`execute stmt1 using @a;`)
	result.Check(testkit.Rows("1"))

	result = tk.MustQuery("select m1.a from t as m1 where m1.a in (1, 3, 5)")
	result.Check(testkit.Rows("1"))

	tk.MustExec("drop table if exists t1")
	tk.MustExec("create table t1 (a float)")
	tk.MustExec("insert t1 values (281.37)")
	tk.MustQuery("select a from t1 where (a in (select a from t1))").Check(testkit.Rows("281.37"))

	tk.MustExec("drop table if exists t1, t2")
	tk.MustExec("create table t1 (a int, b int)")
	tk.MustExec("insert into t1 values (0,0),(1,1),(2,2),(3,3),(4,4)")
	tk.MustExec("create table t2 (a int)")
	tk.MustExec("insert into t2 values (1),(2),(3),(4),(5),(6),(7),(8),(9),(10)")
	result = tk.MustQuery("select a from t1 where (1,1) in (select * from t2 s , t2 t where t1.a = s.a and s.a = t.a limit 1)")
	result.Check(testkit.Rows("1"))
}

func (s *testSuite) TestJoinLeak(c *C) {
	savedConcurrency := plan.JoinConcurrency
	plan.JoinConcurrency = 1
	defer func() {
		plan.JoinConcurrency = savedConcurrency
		s.cleanEnv(c)
		testleak.AfterTest(c)()
	}()
	tk := testkit.NewTestKit(c, s.store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t (d int)")
	tk.MustExec("begin")
	for i := 0; i < 1002; i++ {
		tk.MustExec("insert t values (1)")
	}
	tk.MustExec("commit")
	rs, err := tk.Se.Execute("select * from t t1 left join (select 1) t2 on 1")
	c.Assert(err, IsNil)
	result := rs[0]
	result.Next()
	time.Sleep(100 * time.Millisecond)
	result.Close()
}
