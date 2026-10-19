// Package oracle 是 Oracle 访问能力的分包入口（本包自身无代码），按功能
// 拆为四个子包，仅可被业务系统的存储层（及组装根 main）import：
//
//	conn        连接与数据库访问：Open、DB（GORM()/SQL()/Ping/Close）、
//	            存储过程/函数调用（CallProc/CallFunc 及 CallProcTx 事务变体、
//	            Out/OutCursor、Cursor、Execer、FromGorm）
//	config      基础连接配置：Config、WithDefaults、Validate、BuildDSN
//	base        ORM base 对象：BasePO 审计基类（ID + 创建人/时间 + 修改人/时间，
//	            钩子自动填充，列名全大写对齐 Oracle DDL）
//	repository  仓储存储：Repository[T] 通用审计仓储（Create 自动雪花 ID）
//
// 默认实现基于 godror（CGO，运行期需 Oracle Instant Client）；驱动差异被收敛在
// conn 包内部 driverAdapter 接口之后，未来切换纯 Go 驱动时业务代码零改动。
//
// 快速开始（import 路径 storage/oracle/{config,conn}）：
//
//	db, err := conn.Open(ctx, config.Config{
//	    User: "app", Password: "***", Host: "db.example.com", ServiceName: "ORCLPDB1",
//	})
//	// GORM CRUD：记得 WithContext 传播取消信号
//	err = db.GORM().WithContext(ctx).Create(&order).Error
//	// 原生 SQL
//	rows, err := db.SQL().QueryContext(ctx, "SELECT 1 FROM DUAL")
//	// 存储过程：标量 OUT 与游标 OUT
//	var total int64
//	var cur conn.Cursor
//	err = db.CallProc(ctx, "P_SETTLE", "IN1", conn.Out(&total), conn.OutCursor(&cur))
//	defer cur.Close()
//	for cur.Next() { ... }
//
// 与 GORM 同事务执行存储过程（db-gorm-transactions 约定）：
//
//	err = db.GORM().Transaction(func(tx *gorm.DB) error {
//	    ex, err := conn.FromGorm(tx)
//	    if err != nil { return err }
//	    return conn.CallProcTx(ctx, ex, "P_ACCRUE", arg)
//	})
//
// 审计仓储（import 路径 storage/oracle/{base,repository}）：
//
//	type orderPO struct {
//	    base.BasePO
//	    OrderNo string
//	}
//	repo, err := repository.NewRepository[orderPO](db)
//	err = repo.Create(ctx, &po) // ID 自动雪花，CreatedBy 取 userName/actor 上下文
package oracle
