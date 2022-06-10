### Create

1. <strong>创建记录</strong>：可以获取记录的详细信息，连续创建时，需要注意是否唯一 ID

   ````go
   user := User{Name: "Jinzhu", Age: 18, Birthday: time.Now()}
   
   result := db.Create(&user) // 通过数据的指针来创建
   
   user.ID             // 返回插入数据的主键
   result.Error        // 返回 error
   result.RowsAffected // 返回插入记录的条数
   ````

2. <strong>用指定的字段创建记录</strong>：更新某些字段或忽略传递的字段，选定或者忽略时都要考虑是否重复了主键

   ````go
   // 更新某些字段
   db.Select("Name", "Age", "CreatedAt").Create(&user)
   // INSERT INTO `users` (`name`,`age`,`created_at`) VALUES 
   ("jinzhu", 18, "2020-07-04 11:05:21.775")
   
   // 忽略某些字段
   db.Omit("Name", "Age", "CreatedAt").Create(&user)
   // INSERT INTO `users` (`birthday`,`updated_at`) VALUES 
   ("2020-01-01 00:00:00.000", "2020-07-04 11:05:21.775")
   ````

3. <strong>批量插入</strong>：只需要将 `slice` 传递给 `Create` 即可，过程中钩子方法也会被调用，使用 `CreateInBatches` 可以指定每批数量

4. <strong>创建钩子</strong>：<strong>Hook</strong> 是在创建、查询、更新、删除等操作之前、之后调用的函数，有点 AOP 的味道

   ```go
   func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
     u.UUID = uuid.New()
   
       if u.Role == "admin" {
           return errors.New("invalid role")
       }
       return
   }
   ```

5. <strong>根据 Map 创建</strong>：可以避免一些空值无法查询的情况，可以参考下文的 Query 部分

6. <strong>高级选项</strong>：关联创建、默认值、Upsert 及冲突

---

### Query

该部分比较需要注意的是：GORM 的规范化约束、分页机制、Joins 预加载

1. <strong>检索单个对象</strong>：`First`、`Take`、`Last`，这里一定要<strong>注意主键 `ID` 的规范约束，如果没有，那么按照第一字段排序</strong>

2. <strong>主键检索</strong>：同其他语言的 sql 框架，传入参数查询时，需要特别注意 SQL 注入问题，<strong>如果对象主键已经存在值，那么查询时将绑定</strong>

3. <strong>检索全部对象：</strong>通过调用  `Find` 方法即可

4. <strong>条件</strong>：String 条件、Struct & Map 条件（这里需要注意 GORM 的查询方式以及绑定字段机制）

   > > When querying with struct, GORM will only query with non-zero fields, 
   > >
   > > that means if your field’s value is `0`, `''`, `false` or other [zero values], 
   > >
   > > it won’t be used to build query conditions.

   To include zero values in the query conditions, you can use a map, which will include all key-values as query conditions.

5. <strong>内联条件、Not 条件、Or 条件、Select 选择特定字段、Distinct</strong>：容易理解，参考 demo 即可

6. <strong>Order</strong>：Specify order when retrieving records from the database

7. <strong>Limit & Offset</strong>：`Limit` specify the max number of records to retrieve `Offset` specify the number of records to skip before starting to return the records，Refer to [Pagination](https://gorm.io/zh_CN/docs/scopes.html#pagination) for details on how to make a paginator.<strong>分页机制</strong>

8. <strong>Group By & Having</strong>：条件查询

9. <strong>Joins 预加载</strong>：除去表的关联的作用，有关 Eager Loading 部分，在后面阐述

10. <strong>Scan</strong>：Scanning results into a struct works similarly to the way we use `Find`

    ```go
    db.Raw("select * from user where id=?", 1).Find(&user)
    db.Raw("select * from user where id=?", 1).Scan(&user)
    ```

    在使用Raw自定义SQL查询时，使用Scan来接收数据，虽然Find也是可以接收的，

    但是Find主要还是用来带条件查询的，链接到Raw后面时条件是不起作用的。所以用Scan函数单纯的接收数据就行了。

