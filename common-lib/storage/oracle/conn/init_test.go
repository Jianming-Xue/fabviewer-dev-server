package conn

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func TestInit(t *testing.T) {
	t.Run("无 oracle 段：未启用，返回空句柄与可调用的空 cleanup", func(t *testing.T) {
		db, cleanup, err := Init(map[string]any{"log": map[string]any{}})
		require.NoError(t, err)
		assert.Nil(t, db)
		require.NotNil(t, cleanup)
		cleanup()
	})

	t.Run("空段：零值配置在建连前校验失败", func(t *testing.T) {
		_, _, err := Init(map[string]any{"oracle": map[string]any{}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "User 不能为空")
	})

	t.Run("段存在但字段类型非法：绑定失败", func(t *testing.T) {
		_, _, err := Init(map[string]any{"oracle": map[string]any{"port": "abc"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "绑定失败")
	})

	t.Run("枚举非法：建连前校验失败", func(t *testing.T) {
		_, _, err := Init(map[string]any{"oracle": map[string]any{
			"user": "u", "host": "h", "serviceName": "s", "logLevel": "bogus",
		}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "LogLevel 非法取值")
	})

	t.Run("段存在：camelCase 键绑定、默认值补齐、建连并经 cleanup 关闭", func(t *testing.T) {
		sqlDB, mock := newMockDB(t)
		mock.ExpectQuery(pingQuery).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		mock.ExpectClose().WillReturnError(nil)
		ctrl := gomock.NewController(t)
		drv := NewMockdriverAdapter(ctrl)
		drv.EXPECT().GormDialector(gomock.Any()).
			DoAndReturn(func(dsn string) gorm.Dialector {
				// camelCase 键经忽略大小写绑定；端口零值在 DSN 生成前补默认 1521
				assert.Contains(t, dsn, `user="cfguser"`)
				assert.Contains(t, dsn, "cfghost:1521")
				return connDialector{conn: sqlDB}
			})
		db, cleanup, err := Init(map[string]any{"oracle": map[string]any{
			"user": "cfguser", "password": "p", "host": "cfghost", "serviceName": "svc",
		}}, withDriver(drv))
		require.NoError(t, err)
		require.NotNil(t, db)
		require.NotNil(t, cleanup)
		cleanup()
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
