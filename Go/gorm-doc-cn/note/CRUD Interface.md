# Create

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

# Query

该部分比较需要注意的是：GORM 的规范化约束、分页机制、Joins 预加载

1. <strong>检索单个对象</strong>：`First`、`Take`、`Last`，这里一定要<strong>注意主键 `ID` 的规范约束，如果没有，那么按照第一字段排序</strong>

   * 默认 `First` 如果对象未声明 `ID`，那么将获得第一条记录按照 `ID` 升序排列，

     如果 `Where` 中带有条件，那么将是一个组合条件查询语句，是否包含 `ID` 取决于对象是否有值

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

----

# Advanced Query

1. <strong>Smart Select Fields</strong>: If you often select specific fields with `Select`, 

   you can define a smaller struct for API usage which can select specific fields automatically.

2. <strong>Locking</strong>: GORM supports different types of locks.

3. <strong>Conditions</strong>: You can query with more conditions and subQuery.

4. <strong>In with multiple columns</strong>: Selecting IN with multiple columns.

5. <strong>Named Argument、Find to Map</strong>

6. <strong>FirstOrInit、FirstOrCreate</strong>: This two methods are similar. Besides you can use `Assign` to attributes value to the record.

7. <strong>Scope、Count</strong>: Very frequently used in business 

---

# Update

1. <strong>Save All fields</strong>: `Save` will save all fields when performing the Updating SQL.

2. <strong>Update single column</strong>: When using the `Model` method and its value has a primary value (ID), 
   the primary key will be used to build the condition.

3. <strong>Updates multiple columns</strong>: `Updates` supports update with `struct` or `map[string]interface{}`, 
   when updating with `struct` it will only update non-zero fields by default, you might want to use `map` or `Select` to update.

4. <strong>Update Selected Fields</strong>: you can use `Select`、`Omit` to select or ignore some fields when updating.

5. <strong>Update Hooks</strong>: GORM allows hooks  `BeforeUpdate`,  `AfterUpdate`, those methods will be called when updating a record.

6. <strong>Batch Updates</strong>: If we haven’t specified a record having primary key value with `Model`, GORM will perform a batch updates.

   if you may want to update users with different conditions,  it may does'n work.

7. <strong>BlockGlobalUpdates</strong>: If you perform a batch update without any conditions, GORM won't run it and return error.

   `db.Model(&User{}).Update("name", "jinzhu").Error // gorm.ErrMissingWhereClause`，

   You have to use some conditions or use raw SQL or enable the `AllowGlobalUpdate` mode.

8. <strong>UpdatedRecordsCount</strong>: Get the number of rows affected by a update, `result.RowsAffected`

### Advanced

-----

# Delete

1. <strong>Delete a Record</strong>: When deleting a record, the delete value needs to have primary key combined with other condition.

2. <strong>Delete with primary key:</strong> Using primary key with inline condition, it works with numbers.

3. <strong>Delete Hooks</strong>: Those methods will be called when deleting a record.

4. <strong>Batch Delete</strong>: The specified value has no primary key, GORM will perform a batch delete.

5. <strong>Block Global Delete</strong>: Like Update.

6. <strong>Soft Delete:</strong> The SQL is similar with Update which just make `deleted_at` valued, the record won't be removed from the db.

   But you can find soft deleted records with `Unscoped`

7. <strong>Delete permanently</strong>: You can delete matched records permanently with `Unscoped`.

-----

