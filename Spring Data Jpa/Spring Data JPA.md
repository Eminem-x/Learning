### Spring Data JPA

---

### 1. Preface

<a href="https://docs.spring.io/spring-data/jpa/docs/current/reference/html/#repositories.query-methods.query-lookup-strategies">Links to official documents</a>

Spring Data JPA provides repository support for the Java Persistence API (JPA).

It eases development of applications that need to access JPA data sources.

---

### 2. Working with Spring Data Repositories

1. #### **Core Concepts**

   The central interface in the Spring Data repository abstraction is `Repository`.

   **It takes the domain class to manage as well as the ID type of the domain class as type arguments.**

   **For example：`Repository<T, ID>`** , T is the domain class and ID is the ID type.

   This interface acts primarily as a maker interface to capture the types to work with 

   and to help you to discover interfaces that extends this one.

   ```java
   @Indexed
   public interface Repository<T, ID> {
   }
   ```

   

   The `CrudRepository` interface provides sophisticated CRUD functionality for the entity class that is being managed. 

   ```java
   @NoRepositoryBean
   public interface CrudRepository<T, ID> extends Repository<T, ID> {
       <S extends T> S save(S var1);
   
       <S extends T> Iterable<S> saveAll(Iterable<S> var1);
   
       Optional<T> findById(ID var1);
   
       boolean existsById(ID var1);
   
       Iterable<T> findAll();
   
       Iterable<T> findAllById(Iterable<ID> var1);
   
       long count();
   
       void deleteById(ID var1);
   
       void delete(T var1);
   
       void deleteAll(Iterable<? extends T> var1);
   
       void deleteAll();
   }
   ```

   

   It also provides persistence technology-specific abstractions, such as `JpaRepository` or `MongoRepository`. 

   Those interfaces extend `CrudRepository` and expose the capabilities of the underlying persistence technology 

   in addition to the rather generic persistence technology-agnostic interfaces such as `CrudRepository`.

   ```java
   @NoRepositoryBean
   public interface JpaRepository<T, ID> extends PagingAndSortingRepository<T, ID>, QueryByExampleExecutor<T> {
       List<T> findAll();
   
       List<T> findAll(Sort var1);
   
       List<T> findAllById(Iterable<ID> var1);
   
       <S extends T> List<S> saveAll(Iterable<S> var1);
   
       void flush();
   
       <S extends T> S saveAndFlush(S var1);
   
       void deleteInBatch(Iterable<T> var1);
   
       void deleteAllInBatch();
   
       T getOne(ID var1);
   
       <S extends T> List<S> findAll(Example<S> var1);
   
       <S extends T> List<S> findAll(Example<S> var1, Sort var2);
   }
   ```

   

   On top of the `CrudRepository`, there is a `PagingAndSortingRepository` abstraction 

   that adds additional methods to ease paginated access to entities. 

   ```java
   @NoRepositoryBean
   public interface PagingAndSortingRepository<T, ID> extends CrudRepository<T, ID> {
       Iterable<T> findAll(Sort var1);
   
       Page<T> findAll(Pageable var1);
   }
   ```

2. #### **Query Methods**

   Standard CRUD functionality repositories usually have queries on the underlying datastore.

   With Spring Data, **declaring those queries becomes a four-step process :**

   1. Declare an interface extending Repository or one of its sub-interfaces 

      and type it the domain class and ID type that it should handle.

   2. Declare query methods on the interface.

   3. Set up Spring to create proxy instances for those interfaces, either with JavaConfig or with XML configuration.

   4. Inject the repository instance and use it.

3. #### **Defining Repository Interfaces**

   To define a repository interface, you first need to define a domain class-specific repository interface. 

   The interface must extend `Repository` and be typed to the domain class and an ID type. 

   If you want to expose CRUD methods for that domain type, extend `CrudRepository` instead of `Repository`.

   1. **Fine-tuning Repository Definition**

      Typically, your repository interface extends `Repository` , 

      `CrudRepository` or `PagingAndSortingRepository`.
   
      Alternatively, if you do not want to extend Spring Data interfaces, 
   
      you can also annotate your repository interface with `@RepositoryDefinition`. 

      Extending `CrudRepository` exposes a complete set of methods to manipulate your entities. 

      If you prefer to be selective about the methods being exposed, 
   
      copy the methods you want to expose from `CrudRepository` into your domain repository.
   
      Doing so lets you define your own abstractions on top of the provided Spring Data Repositories functionality.
   
      The intermediate repository interface is annotated with `@NoRepositoryBean`.
   
      Make sure you add that annotation to all repository interfaces 
   
      for which Spring Data should not create instances at runtime.
   
   2. **Using Repositories with Multiple Spring Data Modules**
      
      Using a unique Spring Data module in your application makes things simple, 
      
      because all repository interfaces in the defined scope are bound to the Spring Data module. 
   
      Sometimes, applications require using more than one Spring Data module. 
   
      In such cases, a repository definition must distinguish between persistence technologies. 
      
      When it detects multiple repository factories on the class path,
      
      Spring Data enters strict repository configuration mode. 
      
      Strict configuration uses details on the repository or the domain class
      
      to decide about Spring Data module binding for a repository definition:
      
      1. If the repository definition extends the module-specific repository, 
      
         it is a valid candidate for the particular Spring Data module.
      
      2. If the domain class is annotated with module-specific type annotation,
      
         it is a valid candidate for the particular Spring Data module.
      
         Spring Data modules accept either third-party annotations (such as JPA's `@Entity`) 
      
         or provide their own annotations

         (such as `@Document` for Spring Data MongoDB and Spring Data Elasticsearch).

      **The following example shows a repository that uses module-specific interfaces (JPA in this case):**
   
      ```java
      interface MyRepository extends JpaRepository<User, Long> { }
      
      @NoRepositoryBean
      interface MyBaseRepository<T, ID> extends JpaRepository<T, ID> { … }
      
      interface UserRepository extends MyBaseRepository<User, Long> { … }
      ```
      
      `MyRepository` and `UserRepository` extend `JpaRepository` in their type hierarchy.
      
      They are valid candidates for the Spring Data JPA module.
      
      **The following example shows a repository that uses generic interfaces:**
      
      ```java
      interface AmbiguousRepository extends Repository<User, Long> { … }
      
      @NoRepositoryBean
      interface MyBaseRepository<T, ID> extends CrudRepository<T, ID> { … }
      
      interface AmbiguousUserRepository extends MyBaseRepository<User, Long> { … }
      ```
      
      `AmbiguousRepository` and `AmbiguousUserRepository` extend 
      
      only `Repository` and `CrudRepository` in their type hierarchy. 
   
      While this is fine when using a unique Spring Data module, 
   
      multiple modules cannot distinguish to which particular Spring Data these repositories should be bound.
   
      **The following example show a repository that uses domain classes with annotations:**
   
      ```java
      interface PersonRepository extends Repository<Person, Long> { … }
      
      @Entity
      class Person { … }
      
      interface UserRepository extends Repository<User, Long> { … }
      
      @Document
      class User { … }
      ```
      
      `PersonRepository` references `Person`, which is annotated with the JPA `@Entity` annotation, 
      
      so this repository clearly belongs to Spring Data JPA. 
      
      `UserRepository` references `User`, which is annotated with Spring Data MongoDB's `@Document` annotation.
      
      
      
      **The following bad example shows a repository that uses domain classes with mixed annotations:**
      
      ```java
      interface JpaPersonRepository extends Repository<Person, Long> { … }
      
      interface MongoDBPersonRepository extends Repository<Person, Long> { … }
      
      @Entity
      @Document
      class Person { … }
      ```
      
      This example shows a domain class using both JPA and Spring Data MongoDB annotations. 
      
      It defines two repositories, `JpaPersonRepository` and `MongoDBPersonRepository`. 
      
      One is intended for JPA and the other for MongoDB usage. 
      
      Spring Data is no longer able to tell the repositories apart, which leads to undefined behavior.
      
      
      
      Repository type details and distinguishing class annotations are used for strict repository configuration 
      
      to identify repository candidate for a particular Spring Data module. 
      
      Using multiple persistence technology-specific annotations on the same domain type is possible 
      
      and enables reuse of domain types across multiple persistence technologies. 
      
      However, Spring Data can then no longer determine a unique module with which to bind the repository.
      
      
      
      The last way to distinguish repositories is by scoping repository base packages.
      
      Base packages define the starting points for scanning for repository interface definitions, 
      
      which implies having repository definitions located in the appropriate packages. 
      
      By default, annotation-driven configuration uses the package of the configuration class. 
      
      The base package in XML-based configuration is mandatory.
      
      
      
      **The following example shows annotation-driven configuration of base packages:**
      
      ```java
      @EnableJpaRepositories(basePackages = "com.acme.repositories.jpa")
      @EnableMongoRepositories(basePackages = "com.acme.repositories.mongo")
      class Configuration { … }
      ```
   
4. #### **Define Query Methods**

   The repository proxy has two ways to derive a store-specific query from the method name:

   1. By deriving the query from the method name directly.
   2. By using a manually defined query.

   However, there must be a strategy that decides what actual query is created.

   1. **Query Lookup Strategies**

      Available for the repository infrastructure to resolve the query with XML configuration

   2. **Query Creation**

      The query builder mechanism built into the Spring Data repository infrastructure is useful for 

      building constraining queries over entities of the repository.

      Parsing query method names is divided into subject and predicate.

      The actual result of parsing the method depends on the persistence store for which you create the query.

      However, there are some general things to notice, so consult the appropriate part of your reference documentation.

      ```java
      interface PersonRepository extends Repository<Person, Long> {
      
        List<Person> findByEmailAddressAndLastname(EmailAddress emailAddress, String lastname);
      
        // Enables the distinct flag for the query
        List<Person> findDistinctPeopleByLastnameOrFirstname(String lastname, String firstname);
        List<Person> findPeopleDistinctByLastnameOrFirstname(String lastname, String firstname);
      
        // Enabling ignoring case for an individual property
        List<Person> findByLastnameIgnoreCase(String lastname);
        // Enabling ignoring case for all suitable properties
        List<Person> findByLastnameAndFirstnameAllIgnoreCase(String lastname, String firstname);
      
        // Enabling static ORDER BY for a query
        List<Person> findByLastnameOrderByFirstnameAsc(String lastname);
        List<Person> findByLastnameOrderByFirstnameDesc(String lastname);
      }
      ```

   3. **Property Expressions**

      Property expressions can refer only to a direct property of the managed entity.

      However, you can also define constraints by traversing nested properties.

      ```java
      List<Person> findByAddressZipCode(ZipCode zipCode)
      ```

      Assume a `person` has an `Address` with a `ZipCode`.

      Although this should work for most cases, it is possible for the algorithm to select the wrong property.

      Suppose the `Person` class has an `addressZip` property as well.

      To resolve this ambiguity you can use `_` inside your method name to manually define traversal points.

      `List<Person> findByAddress_ZipCode(ZipCode zipCode)`.

      Because we treat the underscore character as a reserved character,

      we strongly advise following standard Java naming conventions **(Using camel case instead underscores)**.

   4. **Special parameter handling**

      Besides define method parameters, the infrastructure recognizes certain specific types 

      like `Pageable` and `Sort`, to apply pagination and  sorting to your queries dynamically.

      ```java
      Page<User> findByLastname(String lastname, Pageable pageable);
      
      Slice<User> findByLastname(String lastname, Pageable pageable);
      
      List<User> findByLastname(String lastname, Sort sort);
      
      List<User> findByLastname(String lastname, Pageable pageable);
      ```

      APIs taking `Sort` and `Pageable` expect non-null values to be handed into methods.

      If you do not want to apply any sorting or pagination, use `Sort.unsorted()` and `Pageable.unpaged()`. 

      Method lets you pass an `org.springframework.data.domain.Pageable` instance,

      sorting options also need `org.springframework.data.domain.Sort` parameter to your method.

      You can define simple sorting expressions by using property names.

      You can concatenate expressions to collect multiple criteria into one expression. 

      ```java
      Sort sort = Sort.by("firstname").ascending()
        .and(Sort.by("lastname").descending());
      ```

   5. **Limiting Query Results**

      You can limit the results of query methods by using the `first` or `top` keywords,

      which you can use interchangeably.

      You can append an optional numeric value to `top` or `first` to specify the maximum result size to be returned.

      If the number is left out, a result size of 1 is assumed.

      ```java
      User findFirstByOrderByLastnameAsc();
      
      User findTopByOrderByAgeDesc();
      
      Page<User> queryFirst10ByLastname(String lastname, Pageable pageable);
      
      Slice<User> findTop3ByLastname(String lastname, Pageable pageable);
      
      List<User> findFirst10ByLastname(String lastname, Sort sort);
      
      List<User> findTop10ByLastname(String lastname, Pageable pageable);
      ```

   6. **Repository Methods Returning Collections or Iterables**

      Query methods that return multiple results can use standard Java `Iterable`, `List`  and `Set`.

      Beyond that, it supporting returning Spring Data's `Streamable` etc.

   7. **Null Handing of Repository Methods**

   8. **Streaming Query Results**

   9. **Asynchronous Query Results**

5. #### Creating Repository Instances

   This section covers how to create instances and bean definitions for the defined repository interfaces.

   One way to do so is by using the Spring namespace that is shipped with each Spring Data module 

   that supports the repository mechanism, although we generally recommend using Java configuration.

   1. #### XML Configuration

      Each Spring Data module includes a `repositories` element that 

      lets you define a base package that Spring scans for you.

      By default, the infrastructure picks up every interface that extends the persistence technology-specific

      `Repository` sub-interface located under the configured base package and creates a bean instance for it.

      However, you might want more fine-grained control over which interfaces have been instances created for them.

      To do so, use `<include-filter />` and `<extend-filter />` elements.

      ```java
      <repositories base-package="com.acme.repositories">
        <context:exclude-filter type="regex" expression=".*SomeRepository" />
      </repositories>
      ```

   2. #### Java Configuration

      You can also trigger the repository  infrastructure by using a store-specific annotation on a Java configuration class.

      ```java
      @Configuration
      @EnableJpaRepositories("com.acme.repositories")
      class ApplicationConfiguration {
      
        @Bean
        EntityManagerFactory entityManagerFactory() {
          // …
        }
      }
      ```

   3. #### Standalone Usage

      You can also use the repository infrastructure outside of a Spring container.

6. #### Custom Implementations for Spring Data Repositories

   Spring Data provides various  to create query methods with little coding.

   But when those options don't fit your needs you can also provide your own custom implementation for repository methods.

   1. Customizing Individual Repositories
   2. Customize the Base Repository

7. #### Publishing Events from Aggregate Roots

8. #### Spring Data Extensions

