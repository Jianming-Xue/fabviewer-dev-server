// Package oracle 提供 Oracle 访问封装：GORM ORM（DB.GORM 出口）、原生 SQL
// （DB.SQL 出口）与存储过程/函数调用（DB.CallProc/CallFunc 及 CallProcTx 等
// 事务变体），仅可被业务系统的存储层（及组装根 main）import。
//
// 默认实现基于 godror（CGO，运行期需 Oracle Instant Client）；驱动差异被收敛在
// 内部 driverAdapter 接口之后，未来切换纯 Go 驱动时业务代码零改动。
//
// 快速开始：
//
//	db, err := oracle.Open(ctx, oracle.Config{
//	    User: "app", Password: "***", Host: "db.example.com", ServiceName: "ORCLPDB1",
//	})
//	// GORM CRUD：记得 WithContext 传播取消信号
//	err = db.GORM().WithContext(ctx).Create(&order).Error
//	// 原生 SQL
//	rows, err := db.SQL().QueryContext(ctx, "SELECT 1 FROM DUAL")
//	// 存储过程：标量 OUT 与游标 OUT
//	var total int64
//	var cur oracle.Cursor
//	err = db.CallProc(ctx, "P_SETTLE", "IN1", oracle.Out(&total), oracle.OutCursor(&cur))
//	defer cur.Close()
//	for cur.Next() { ... }
//
// 与 GORM 同事务执行存储过程（db-gorm-transactions 约定）：
//
//	err = db.GORM().Transaction(func(tx *gorm.DB) error {
//	    ex, err := oracle.FromGorm(tx)
//	    if err != nil { return err }
//	    return oracle.CallProcTx(ctx, ex, "P_ACCRUE", arg)
//	})
package oracle
