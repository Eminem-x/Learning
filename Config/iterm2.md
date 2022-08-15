# Iterm2安装配置

> 同学给我推荐一篇 <a href="https://baijiahao.baidu.com/s?id=1695767365094237852&wfr=spider&for=pc">Iterm2 的安装配置</a>，不过担心以后文章丢失，并且改了一些配置，便整理备用

现在最主流的方式就是用 `Oh My ZSH` + `Powerline字体` + `agnoster主题` + `zsh-syntax-highlighter语法高亮`，

`Oh My ZSH `是一个开源的 `ZSH` 管理配置工具，因为默认的 `ZSH` 配置起来比较麻烦，此开源工具也是目前最主流的 `ZSH ` 配置方式，

`Powerline字体` 也是必不可少的工具，因为有一些主题使用了一些特殊字体，如果没有安装字体的话有些符号就会乱码。

### Iterm2 安装

`brew install iterm2` 即可，而后将其设置为默认 `terminal`，即可使用。

 <img src="https://github.com/Eminem-x/Learning/blob/main/Config/pic/iterm2/%E9%85%8D%E7%BD%AEitem.png" alt="system call" style="max-width: 60%">

### Oh My ZSH 安装

通过 `curl` 或者 `wget` 均可：

````shell
sh -c "$(curl -fsSL https://raw.github.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
或者
sh -c "$(wget https://raw.github.com/ohmyzsh/ohmyzsh/master/tools/install.sh -O -)"
````

打开用户目录 `~/.zshrc`，把 `ZSH_THEME` 修改为 `agnoster`（或者 `robbyrussell`），保存即可。

然后从 `Profiles>Open Profiles>Edit Profiles>Colors`，找到右下角修改主题的地方，随意选择一个自己喜欢的主题即可。

 <img src="https://github.com/Eminem-x/Learning/blob/main/Config/pic/iterm2/%E9%85%8D%E7%BD%AE%E4%B8%BB%E9%A2%98.png" alt="system call" style="max-width: 60%">

这样设置完成后，终端大概变成下图所示：

 <img src="https://github.com/Eminem-x/Learning/blob/main/Config/pic/iterm2/%E6%95%88%E6%9E%9C.jpg" alt="system call" style="max-width: 60%">

### Powerline 字体安装

其实，做完上面的步骤之后，你会发现其实出现乱码了，我的截图是因为我已经做完了这些步骤。

解决乱码的方式是下载`Powerline`字体。

可以通过命令行安装，当然也可以直接去`github`手动下载安装。

```shell
git clone https://github.com/powerline/fonts.git --depth=1
cd fonts
./install.sh
```

从`Profiles>Open Profiles>Edit Profiles>Text`修改字体。红色矩形框勾选，字体其实有好几个，

至于用哪个`Powerline`结尾的都行，只是大小和间距略有不同，个人更喜欢 `Source Code Pro` 多一些。

 <img src="https://github.com/Eminem-x/Learning/blob/main/Config/pic/iterm2/%E5%AD%97%E4%BD%93.png" alt="system call" style="max-width: 60%">

### 安装语法高亮

`brew install zsh-syntax-highlighting`

然后打开 `~/.zshrc`，找到 `plugins` 位置修改为以下内容

```shell
# Which plugins would you like to load?
# Standard plugins can be found in $ZSH/plugins/
# Custom plugins may be added to $ZSH_CUSTOM/plugins/
# Example format: plugins=(rails git textmate ruby lighthouse)
# Add wisely, as too many plugins slow down shell startup.
plugins=(git zsh-syntax-highlighting)
```

同时在文件末尾添加（**实际操作中发现不需要添加**）

`source /usr/local/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh`

保存退出后，执行命令，重启终端运行即可：

````shell
git clone https://github.com/zsh-users/zsh-syntax-highlighting.git ~/.oh-my-zsh/custom/plugins/zsh-syntax-highlighting
````

### 安装自动补全

这个自动补全的内容是历史输入的命令，使用方向右键 `->` 补全，使用上述同样的方式安装。

```shell
git clone https://github.com/zsh-users/zsh-autosuggestions ~/.oh-my-zsh/custom/plugins/zsh-autosuggestions
```

在 `~/.zshrc` 文件中配置插件，然后重启生效：

```shell
git clone https://github.com/zsh-users/zsh-autosuggestions ~/.oh-my-zsh/custom/plugins/zsh-autosuggestions
```

