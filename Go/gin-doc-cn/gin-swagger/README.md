### Preface

在现如今前后端分离的模式下，前端如何快捷准确地确定接口类型、请求参数以及响应结构体，是开发中不可或缺的步骤，

与此同时，后端如何相应地即使反馈，也很重要，而 `swagger` 解决了这个问题，方便了前后端的联调测试，

接下来的内容主要是：在 `Go` 中使用 `Gin` 框架如何接入 `swagger` 服务，以及简易的代码包组织结构，

具体代码参考仓库：https://github.com/Eminem-x/Learning/tree/main/Go/gin-doc-cn/gin-swagger

### Details

#### 安装 `swag`

```bash
go get -u github.com/swaggo/swag/cmd/swag
```

#### 校验是否安装成功

```bash
swag -v
```

#### 在项目中安装 `gin-swagger` 依赖

```bash
go get -u github.com/swaggo/gin-swagger
go get -u github.com.swaggo/gin-swagger/swaggerFiles // 如果报错，不影响使用
```

#### 添加配置注解

具体路径，参考仓库代码即可

````go
package router

import (
	"gin-swagger/services"
	_ "gin-swagger/swagger/docs" // swagger 生成成功后, 添加进来
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"log"
)

func InitRouter() {
	r := gin.Default()
	setRouter(r)

	// Run the server
	if err := r.Run(":8000"); err != nil {
		log.Panicf("startup service failed, err:%v\n", err)
	}
}

// Test swagger

// @title Swagger API
// @version 1.0
// @description This is a api server for test.
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:8000
// @BasePath /
// @query.collection.format multi
// @securityDefinitions.basic BasicAuth
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @securitydefinitions.oauth2.application OAuth2Application
// @tokenUrl https://example.com/oauth/token
// @scope.write Grants write access
// @scope.admin Grants read and write access to administrative information
// @securitydefinitions.oauth2.implicit OAuth2Implicit
// @authorizationurl https://example.com/oauth/authorize
// @scope.write Grants write access
// @scope.admin Grants read and write access to administrative information
// @securitydefinitions.oauth2.password OAuth2Password
// @tokenUrl https://example.com/oauth/token
// @scope.read Grants read access
// @scope.write Grants write access
// @scope.admin Grants read and write access to administrative information
// @securitydefinitions.oauth2.accessCode OAuth2AccessCode
// @tokenUrl https://example.com/oauth/token
// @authorizationurl https://example.com/oauth/authorize
// @scope.admin Grants read and write access to administrative information
// @x-extension-openapi {"example": "value on a json format"}
func setRouter(r *gin.Engine) {
	// 必须在 router 中引入, 不然访问 swagger 会报 404 错误
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/login", services.Login)
}
````

注意以下的代码必须在 `router` 中引入，不然访问 `swagger` 会报 404

```go
r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
```

#### 注释接口

详细的参数配置参考：https://github.com/swaggo/swag#general-api-info

示例代码（封装请求体、响应体、序列化）：

````go
// Login
// @Summary Login
// @Tags Admin
// @Produce json
// @Success 200 {object} pkg.SuccessResponse{data=serializers.LoginResponse{login=serializers.Login}}
// @Failure 404,500 {object} pkg.ErrorResponse
// @Router /login [get]
func Login(c *gin.Context) {
	_, _ = c.Writer.WriteString("Login!")

	// 通常需要封装好 response 和 serializer
	c.JSON(http.StatusOK, pkg.SuccessResponse{
		Status: "200 OK",
		Data:   serializers.SerializeLogin(),
	})
}
````

#### 生成 `swagger` 文档

```bash
-o 指定文档生成目录
-g 指定 swagger-info 所在的目录 默认 main.go
--parseDependency 解析外部依赖文件夹中的 go 文件，默认禁用，一般是在 init 生成文件时提示无法识别 struct 时添加

swag init -g router/router.go -o swagger/docs --parseDependency
rm -rf swagger && swag init -g router/router.go -o swagger/docs --parseDependency
```

生成 `swagger` 文档之后再在 `router.go` 文件中引入，引入下面内容：

```go
_ "gin-swagger/swagger/docs" // swagger 生成成功后, 添加进来
```

#### 访问 `swagger` 界面

完成上述步骤之后启动项目，然后通过 `http://localhost:port/swagger/index.html` 访问即可