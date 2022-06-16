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

4. <strong>创建钩子</strong>：<strong>Hook</strong> 是在创建、查询、更新、删除等操作之前、之后调用的函数，有点 AOP 的味道,

   当批量操作时，如果存在一条 record 失败了，那么都会失败，如果想跳过钩子方法，可以使用 `SkipHooks` 会话模式。

   ```go
   func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
     u.UUID = uuid.New()
   
       if u.Role == "admin" {
           return errors.New("invalid role")
       }
       return
   }
   ```

5. <strong>根据 Map 创建</strong>：可以避免一些空值无法查询的情况，可以参考下文的 Query 部分，实现了这部分 demo，发现存在差异。

   没有明白 `Note` 中无法自动填充主键的含义：https://stackoverflow.com/questions/72597438/gorm-create-from-map

   comment 给出了解答，通过测试发现确实如此：back filled 含义是不会为 map 对象

6. <strong>使用 SQL 表达式、Context Valuer 创建记录</strong>：Learn it when needed.

7. <strong>高级选项</strong>：`upsert` ：插入一行唯一数据时，若该行数据已存在，则更新该行内容，不存在则插入新的一行

   * <strong>关联创建</strong>：创建关联数据时，如果关联值非零，那么这些关联会被 `upsert`，并且它们的 `Hook` 方法也会被调用，

     可以通过 `Select`、`Omit` 跳过关联保存。

   * <strong>默认值</strong>：需要注意如果插入 `0`，`''`, `false`，会被默认值覆盖，可以采用指针类型或 Scanner/Valuer 来避免这个问题

   * <strong>Upsert 及冲突：</strong>GORM 为不同数据库提供了兼容的 Upsert 支持

---

### Query

该部分比较需要注意的是：GORM 的规范化约束、分页机制、Joins 预加载

1. <strong>检索单个对象</strong>：`First`、`Take`、`Last`，这里一定要<strong>注意主键 `ID` 的规范约束，如果没有，那么按照第一字段排序</strong>

   * 默认 `First` 是按照 `ID` 查询不参考其他字段，但如果对象未声明 `ID`，那么将获得第一条记录按照 `ID` 升序排列
   * `Take` 方法在测试后发现，如果对象最初有 `ID`，那么按照主键查询，否则 `Get one record, no specified order`
   * `...or when the model is specified using db.Model()` 用于将结果传递给一个最初非 `model` 的对象
   * `no primary key defined, results will be ordered by first field (i.e., code)`，最好还是遵守规定约束

2. <strong>主键检索</strong>：同其他语言的 sql 框架，传入参数查询时，需要特别注意 SQL 注入问题，<strong>如果对象主键已经存在值，那么查询时将绑定</strong>

3. <strong>检索全部对象：</strong>通过调用  `Find` 方法即可，`Select("*").Scan(&model)` 也可以，不过多此一举

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

9. <strong>Joins 预加载</strong>：除去表的关联的作用，预加载可以是无关的表，也可以是派生表，具体方式可参考官网例子

10. <strong>Scan</strong>：Scanning results into a struct works similarly to the way we use `Find`

    ```go
    db.Raw("select * from user where id=?", 1).Find(&user)
    db.Raw("select * from user where id=?", 1).Scan(&user)
    ```

    在使用Raw自定义SQL查询时，使用Scan来接收数据，虽然Find也是可以接收的，

    但是Find主要还是用来带条件查询的，链接到Raw后面时条件是不起作用的。所以用Scan函数单纯的接收数据就行了。

    > Correct me if I'm wrong but Find is used to automatically use the model to select. 
    >
    > Something like db.Find(&ModelName) will be the same as select * from model_name.
    >
    > Meanwhile Scan is used if you do raw query where you do 
    >
    > "select field_one, b+c as field_two, c as field_three from table_name" 
    >
    > then map the result to a custom model depending on your query.

