# SpaceVim安装配置

推荐阅读：https://spacevim.org/cn/documentation/

配置Go：https://spacevim.org/use-vim-as-a-go-ide/

### 安装流程

```bash
brew install nvim
curl -sLf https://spacevim.org/install.sh | bash
nvim
```

然后替换 `~/.SpaceVim.d/init.toml` 中内容如下即可：

```shell
#=============================================================================
# dark_powered.toml --- dark powered configuration example for SpaceVim
# Copyright (c) 2016-2022 Wang Shidong & Contributors
# Author: Wang Shidong < wsdjeg@outlook.com >
# URL: https://spacevim.org
# License: GPLv3
#=============================================================================

# All SpaceVim option below [option] section
[options]
    # set spacevim theme. by default colorscheme layer is not loaded,
    # if you want to use more colorscheme, please load the colorscheme
    # layer
    colorscheme = "SpaceVim"
    colorscheme_bg = "dark"
    # Disable guicolors in basic mode, many terminal do not support 24bit
    # true colors
    enable_guicolors = true
    # Disable statusline separator, if you want to use other value, please
    # install nerd fonts
    statusline_separator = "arrow"
    statusline_iseparator = "arrow"
    buffer_index_type = 4
    enable_tabline_filetype_icon = true
    enable_statusline_mode = false
    # 目录树左边显示
    filetree_direction = "left"
    # 禁止检查升级
    checkinstall = false

# Enable autocomplete layer
[[layers]]
name = 'autocomplete'
auto_completion_return_key_behavior = "complete"
auto_completion_tab_key_behavior = "cycle"

[[layers]]
name = 'shell'
default_position = 'top'
default_height = 30

[[layers]]
  name = "colorscheme"

[[layers]]
  name = "lang#go"
  format_on_save = true

[[layers]]
  name = "format"

# This is an example for adding custom plugins lilydjwg/colorizer
[[custom_plugins]]
    repo = "lilydjwg/colorizer"
    merged = false
```

最后运行 `nvim` 即可，或者 `vim` 即可，具体的操作参考文章开头推荐阅读的链接。