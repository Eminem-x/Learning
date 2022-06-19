# Mac 用 brew 安装 MySQL

## 一、通过 brew 命令安装

### 安装 brew

参考文档：https://github.com/Eminem-x/Learning/blob/main/Config/brew.md

### 安装 MySQL

1. 终端输入命令：`brew install mysql`

2. mysql 说明了 mysql 的密码以及启动方式

   ````bash
   ==> mysql
   We've installed your MySQL database without a root password. To secure it run:
       mysql_secure_installation
   
   MySQL is configured to only allow connections from localhost by default
   
   To connect run:
       mysql -uroot
   
   To restart mysql after an upgrade:
     brew services restart mysql
   Or, if you don't want/need a background service you can just run:
     /opt/homebrew/opt/mysql/bin/mysqld_safe --datadir=/opt/homebrew/var/mysql
   ````

3. 启动 mysql：可以使用命令查看提示，`brew info mysql`，根据提示，有两种启动方式：

   `brew services start mysql` 或者 `mysql.server start`

## 二、启动 MySQL 并设置密码

### 1. 启动 MySQL

输入：`mysql.server start`，启动，提示成功

```bash
DIDI-C02G17HXQ05P:~ didi$ mysql.server start
Starting MySQL
 SUCCESS!
```

### 2. 设置密码

输入：`mysql_secure_installation`，设置密码，此处步骤为操作实例：

```bash
总结：
1.启动服务
2.执行安全配置
3.删除匿名帐号，放开远程连接，删除test测试库，重新加载权限表
4.测试连接

//输入命令
DIDI-C02G17HXQ05P:~ didi$ mysql_secure_installation

Securing the MySQL server deployment.

Connecting to MySQL using a blank password.

//验证密码组件可用于测试密码
改善安全。它检查密码的强度
并允许用户仅设置以下密码：
足够安全。是否要设置验证密码组件？
VALIDATE PASSWORD COMPONENT can be used to test passwords
and improve security. It checks the strength of password
and allows the users to set only those passwords which are
secure enough. Would you like to setup VALIDATE PASSWORD component?

Press y|Y for Yes, any other key for No: yes  //输入yes表示设置密码

There are three levels of password validation policy:

//提示密码强度为 low、medium、strong 低中高三种
LOW    Length >= 8
MEDIUM Length >= 8, numeric, mixed case, and special characters
STRONG Length >= 8, numeric, mixed case, special characters and dictionary                  file

//输入自己想要设置的密码强度等级
Please enter 0 = LOW, 1 = MEDIUM and 2 = STRONG: 2
Please set the password for root here.

//新密码(输入不可见)
New password:Qian123@@@
//再次确认
Re-enter new password: Qian123@@@

Estimated strength of the password: 100
Do you wish to continue with the password provided?(Press y|Y for Yes, any other key for No) : y
By default, a MySQL installation has an anonymous user,
allowing anyone to log into MySQL without having to have
a user account created for them. This is intended only for
testing, and to make the installation go a bit smoother.
You should remove them before moving into a production
environment.

//是否删除匿名用户，y表示yes 其他表示no（百度建议删除：在mysql刚刚被安装后，存在用户名、密码为空的用户。这使得数据库服务器有无需密码被登录的可能性。为消除隐患，将匿名用户删除。）
Remove anonymous users? (Press y|Y for Yes, any other key for No) : y
Success.

Normally, root should only be allowed to connect from
'localhost'. This ensures that someone cannot guess at
the root password from the network.

//不允许根用户远程登录？
Disallow root login remotely? (Press y|Y for Yes, any other key for No) : n

 ... skipping.
By default, MySQL comes with a database named 'test' that
anyone can access. This is also intended only for testing,
and should be removed before moving into a production
environment.

//删除测试数据库并访问他？
Remove test database and access to it? (Press y|Y for Yes, any other key for No) : n

 ... skipping.
Reloading the privilege tables will ensure that all changes
made so far will take effect immediately.

//现在重新加载特权表吗？
Reload privilege tables now? (Press y|Y for Yes, any other key for No) : y
Success.

All done!

//测试数据库连接，输入命令mysql -uroot -p后输入设置的密码
DIDI-C02G17HXQ05P:~ didi$ mysql -uroot -p
Enter password:
Welcome to the MySQL monitor.  Commands end with ; or \g.
Your MySQL connection id is 12
Server version: 8.0.28 Homebrew

Copyright (c) 2000, 2022, Oracle and/or its affiliates.

Oracle is a registered trademark of Oracle Corporation and/or its
affiliates. Other names may be trademarks of their respective
owners.

Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

//查看表
mysql> show databases;
+--------------------+
| Database           |
+--------------------+
| information_schema |
| mysql              |
| performance_schema |
| sys                |
+--------------------+
4 rows in set (0.00 sec)
```

至此 MySQL 安装成功，在系统中存在 MySQL

### 3. 卸载 MySQL

```bash
sudo rm /usr/local/mysql
sudo rm -rf /usr/local/mysql*
sudo rm -rf /Library/StartupItems/MySQLCOM
sudo rm -rf /Library/PreferencePanes/My*
rm -rf ~/Library/PreferencePanes/My*
sudo rm -rf /Library/Receipts/mysql*
sudo rm -rf /Library/Receipts/MySQL*
sudo rm -rf /var/db/receipts/com.mysql.*
```

依次执行完以上命令后，检查 MySQL 是否还在 `系统-系统偏好设置` 内展示，文件内是否有残留

## 三、可视化工具 Navicat

需要购买，但是如果是学生的话可以申请一年的免费使用资格：http://www.navicat.com.cn

