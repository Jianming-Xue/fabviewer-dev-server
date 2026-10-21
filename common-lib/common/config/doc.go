// Package config 提供业务服务配置文件的统一加载与读取能力：每个业务服务
// 维护自己的 application.yml，本包负责加载、环境变量覆盖与按需绑定到
// 各组件的 Config 结构体。
//
// 加载：LoadConfig(appName) 读取环境变量 {appName}_CFG 指向的配置文件
// （缺省为工作目录下的 application.yml），支持 yml/yaml/json 三种格式，
// 解析为嵌套 map[string]any（如 log.level: "info" → {"log": {"level":
// "info"}}，任意深度同原理）。加载后每个叶子值可被同名环境变量覆盖
// （如 security.log.level 覆盖 log.level 下的 level），未设置则保留文件
// 原值；覆盖值为字符串原样进入 map，类型转换推迟到绑定阶段。
//
// 读取：Get 返回路径下的原始子 map（未命中为 nil，不报错）；GetSection[T]
// 将子 map 绑定到目标结构体（字段匹配：mapstructure tag 优先，无 tag 字段
// 按完整字段名忽略大小写；开启 WeaklyTypedInput，环境变量覆盖产生的字符串
// 可落到数字/布尔字段）；ValidateIsEmpty/IsNumber/IsBoolean 对指定 key 的
// 值做断言式校验（对字符串值同样按宽松规则判断，与环境变量覆盖机制一致）。
//
// 本包只提供加载与读取能力，不内置任何业务配置项；各组件（logx、
// storage/* 等）自行提供从 map 初始化自己的方法，是否启用由组件自己判断。
package config
