> I strongly recommend that read english version in this part, because there are many translation errors in Chines.

# Belongs To

### Overview

A `belongs to` association sets up a one-to-one connection with another model,

such that each instance of the declaring model "belongs to" one instance of the other model,

For example, if your application includes users and companies, and each user can be assigned to exactly one company,

the following represent that relationship.

```go
// `User` belongs to `Company`, `CompanyID` is the foreign key
type User struct {
  gorm.Model
  Name      string
  CompanyID int
  Company   Company
}

type Company struct {
  ID   int
  Name string
}
```

通过建表语句，就不难看出 belongs to 的含：一个 user 属于一个 company，user 中存储 company 的主键也就是 ID，

所以可以理解成一个用户属于一个公司，这与传统的外键定义是相同的，也会遵循一些约束和重写。

### Override Foreign Key

To define a belongs to relationship, the foreign key must exist, 

the default foreign key uses the owner's type name plus its primary field name.

GORM provides a way to customize the foreign key, for example:

```go
type User struct {
  gorm.Model
  Name         string
  CompanyRefer int
  Company      Company `gorm:"foreignKey:CompanyRefer"`
  // use CompanyRefer as foreign key
}

type Company struct {
  ID   int
  Name string
}
```

默认的外键关系就是 `tableName + ID` ，但是你可以重定义外键的名称在 GORM 中，但是我认为这是糟糕的，代码可读性降低。

### Override References

For a belongs to relationship, GORM usually uses the owner's primary field as the foreign key's value.

When you assign a user to a company, GORM will save the company's ID into the user's `CompanyID` field.

```go
type User struct {
  gorm.Model
  Name      string
  CompanyID string
  Company   Company `gorm:"references:Code"` // use Code as references
}

type Company struct {
  ID   int
  Code string
  Name string
}
```

这里如果按照官方文档操作，是会报错的，可能错误有以下两个：

```
Error 1170: BLOB/TEXT column 'company_id' used in key specification without a key length
Error 1822: Failed to add the foreign key constraint. Missing index for constraint 'fk_users_company' in the referenced table 'companys'
```

强烈建议官方文档中的 demo 都自己尝试一遍，因为很多地方都存在 error，上述的原因在于：

1. 没有对 `string` 类型加以长度限制
2. 作为外键的字段在主表中不是 `unique` 的，需要主键或者 `unique` 索引限制

正确的声明方式应该如下：

```go
type Student struct {
	gorm.Model
	Name     string
	SchoolID string
	// Foreign Key Constraints
	School School `gorm:"references:Code"`
}

type School struct {
	ID   int
	Code string `gorm:"unique;type:varchar(32)"`
	Name string
}
```

### Foreign Key Constraints

You can setup `OnUpdate`, `OnDelete` constaints with tag `constraint`, it will be created when migrating with GORM.

```go
type User struct {
  gorm.Model
  Name      string
  CompanyID int
  Company   Company `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

type Company struct {
  ID   int
  Name string
}
```

这部分是关于外键约束的介绍，可以自己写两个数据测试一下，级联更新和删除。

### CRUD with Belongs To、Eager Loading

These two parts are detailed in the `Assocation Mode` and `Preload` sections.