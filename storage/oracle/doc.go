// Package oracle 提供 Oracle 访问封装：GORM ORM（GORM 出口）、原生 SQL（SQL 出口）
// 与存储过程/函数调用（CallProc/CallFunc 及事务变体），仅可被业务系统的存储层
// （及组装根 main）import。
//
// 默认实现基于 godror（CGO，运行期需 Oracle Instant Client）；驱动差异被收敛在
// 内部 driverAdapter 接口之后，未来切换纯 Go 驱动时业务代码零改动。
//
// 快速开始：
//
//	db, err := oracle.Open(ctx, oracle.Config{
//	    User: "app", Password: "***", Host: "db.example.com", ServiceName: "ORCLPDB1",
//	})
//	var n int64
//	err = db.CallProc(ctx, "P_ACCRUE", "arg1", oracle.Out(&n))
package oracle
