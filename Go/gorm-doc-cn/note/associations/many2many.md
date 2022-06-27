> `Has Many` is similar with `Has One`, so don't make up.

# Many To Many

Many to Many add a join table between two models which is very common and useful.

```go
// User has and belongs to many languages, `user_languages` is the join table
type User struct {
  gorm.Model
  Languages []Language `gorm:"many2many:user_languages;"`
}

type Language struct {
  gorm.Model
  Name string
}
```

Besides you can add `Back-Reference` with Gorm declared.

```go
// User has and belongs to many languages, use `user_languages` as join table
type User struct {
  gorm.Model
  Languages []*Language `gorm:"many2many:user_languages;"`
}

type Language struct {
  gorm.Model
  Name string
  Users []*User `gorm:"many2many:user_languages;"`
}
```

可以创建多对多的关系，并且省去一个中间表的 model，这点非常重要，只有开发过才知道这一步的精简之处，

而 `Back-Reference` 的含义就是在 mode 中彼此都含有对方，这一步看实际开发需要。

### Retrieve

I suggest that this part coded by yourself. 因为会发现出现很多空数据以及需要自己去整理数据。

### Self-Referential Many2Many

```go
type User struct {
  gorm.Model
  Friends []*User `gorm:"many2many:user_friends"`
}

// Which creates join table: user_friends
//   foreign key: user_id, reference: users.id
//   foreign key: friend_id, reference: users.id
```

上面这部分是目前开发中我所遇到的，基础但是重要，而剩下的部分比如：外键重写约束、复合外键、自定义连接表，

简略地浏览过后，有了大概印象，对于 ORM 框架，更多的是要在开发中总结学习，这是需要经验的。