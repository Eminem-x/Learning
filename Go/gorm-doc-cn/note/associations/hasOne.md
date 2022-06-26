# Has One

A `has one` association sets up a one-to-one connection with another model, 

but with somewhat different semantics(and consequences).

This association indicates that each instance of a model contains or possesses one instance of another model.

我认为 has one 的概念是容易和 belongs to 混淆的，但是从建表语句以及情景分析还有字段不同，可以体会二者差异，

前者是拥有或者包含，后者是属于，并且建表中可以发现前者是不包含外键关系的，<strong>但是仍为一对一的关系！</strong>。

### Declare

````go
// User has one CreditCard, CreditCardID is the foreign key
type User struct {
  gorm.Model
  CreditCard CreditCard
}

type CreditCard struct {
  gorm.Model
  Number string
  UserID uint
}
````

Pay attention to the `UserID` field which is necessary.

### Retrieve

```go
// Retrieve user list with edger loading credit card
func GetAll(db *gorm.DB) ([]User, error) {
  var users []User
  err := db.Model(&User{}).Preload("CreditCard").Find(&users).Error
  return users, err
}
```

通过使用 `Preload` 可以加载出包含表的 record 详细信息

### Override Foreign Key、Override References

This part is similar with `belongs to`.

### Polymorphism Association

GORM supports polymorphism association for `has one` and `has many`, 

it will save owned entity’s table name into polymorphic type’s field, primary key into the polymorphic field

关于多态关系这一句话的描述，不要看中文版的翻译，简直是灾难，实际上表达语义是：

他将保存拥有实体的表明在多态类型的字段，主键保存在多态字段，实际上最好是看建表语句以及 model 的声明：

````go
type Cat struct {
  ID    int
  Name  string
  Toy   Toy `gorm:"polymorphic:Owner;"`
}

type Dog struct {
  ID   int
  Name string
  Toy  Toy `gorm:"polymorphic:Owner;"`
}

type Toy struct {
  ID        int
  Name      string
  OwnerID   int
  OwnerType string
}
````

You can change the polymorphic type value with tag `polymorphicValue`.

可以通过简单的 demo 测试一下创建过程，就会理解 ID 是用来存储前者的 ID，而 Type 用来记录前者的 model 类型，可自定义。

### Self-Referential Has One、FOREIGN KEY Constraints