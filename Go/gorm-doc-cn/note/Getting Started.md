## 声明模型

### 模型定义及约定

GORM 倾向于约定，而不是配置，遵循 GORM 已有的约定，可以减少配置和代码量。默认情况下，GORM 使用 `ID` 作为主键，

使用结构体名的 `蛇形复数` 作为表名，字段名的 `蛇形` 作为列名，并使用 `CreatedAt`、`UpdatedAt` 字段追踪创建、更新时间。

以<strong>嵌入结构体</strong>的方式嵌入到结构体中，以包含这几个字段：

```go
// gorm.Model 的定义
type Model struct {
  ID        uint           `gorm:"primaryKey"`
  CreatedAt time.Time
  UpdatedAt time.Time
  DeletedAt gorm.DeletedAt `gorm:"index"`
}

// 普通模型定义
type User struct {
  ID           uint
  Name         string
  Email        *string
  Age          uint8
  Birthday     *time.Time
  MemberNumber sql.NullString
  ActivatedAt  sql.NullTime
  CreatedAt    time.Time
  UpdatedAt    time.Time
}
```

### 高级选项

字段级权限控制：使得一个字段的权限是只读、只写、只创建、只更新或者被忽略

```go
type User struct {
  Name string `gorm:"<-:create"` // allow read and create
  Name string `gorm:"<-:update"` // allow read and update
  Name string `gorm:"<-"`        // allow read and write (create and update)
  Name string `gorm:"<-:false"`  // allow read, disable write permission
  Name string `gorm:"->"`        // readonly (disable write permission unless it configured)
  Name string `gorm:"->;<-:create"` // allow read and create
  Name string `gorm:"->:false;<-:create"` // createonly (disabled read from db)
  Name string `gorm:"-"`            // ignore this field when write and read with struct
  Name string `gorm:"-:all"`        // ignore this field when write, read and migrate with struct
  Name string `gorm:"-:migration"`  // ignore this field when migrate with struct
}
```

创建/更新时间追踪：`gorm.model` 中的字段追踪时间，更新时会自动填充时间，也可以根据需要额外配置

````go
type User struct {
  CreatedAt time.Time // 在创建时，如果该字段值为零值，则使用当前时间填充
  UpdatedAt int       // 在创建时该字段值为零值或者在更新时，使用当前时间戳秒数填充
  Updated   int64 `gorm:"autoUpdateTime:nano"` // 使用时间戳填纳秒数充更新时间
  Updated   int64 `gorm:"autoUpdateTime:milli"` // 使用时间戳毫秒数填充更新时间
  Created   int64 `gorm:"autoCreateTime"`      // 使用时间戳秒数填充创建时间
}
````

嵌入结构体：对于匿名字段可以直接嵌入比如：`gorm.model`，对于正常结构体：`embedded` 标签嵌入，也可增加前缀

```go
type Author struct {
    Name  string
    Email string
}

type Blog struct {
  ID      int
  Author  Author `gorm:"embedded"`
  Upvotes int32
}
// 等效于
type Blog struct {
  ID    int64
  Name  string
  Email string
  Upvotes  int32
}
```

字段标签：对字段进一步说明，比如常用的：`comment`、`unique`、`primaryKey`

关联标签：`GORM` 允许通过标签为关联配置外键、约束、`many2many` 表等

### 数据库连接

`GORM` 官方支持的数据库类型有： MySQL, PostgreSQL, SQlite, SQL Server

和其他技术一样，采用驱动+连接池技术，需要进行配置，配置过程参考代码即可。

```go
dsn := "username:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
// https://cloud.tencent.com/developer/article/1830807 gorm 输出执行的 sql
db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
if err != nil {
	panic("failed to connect database")
}
```