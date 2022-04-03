#include "kernel/types.h"
#include "kernel/stat.h"
#include "user/user.h"
#include "kernel/param.h"

#define MAXN 1024

int main(int argc, char *argv[])
{
    if (argc < 2)
    {
        fprintf(2, "usage: xargs command\n");
        exit(1);
    }

    char *childArgv[MAXARG]; // 存放子进程 exec 的参数
    // 以 | 为界限, 前面内容存放在 stdin , 略去 xargs
    for (int i = 1; i < argc; i++)
    {
        childArgv[i - 1] = argv[i];
    }

    char buf[MAXN]; // 存放 stdin 转化而来的参数
    char ch = 0;    // 存储从从 stdin 读取的字符
    int stat = 1;   // 从 stdin 中 read 返回的状态

    while (stat) // stdin 中还有数据
    {
        int buf_cnt = 0;   // buf 尾指针
        int arg_begin = 0; // 当前这个参数在 buf 中开始的位置
        int argv_cnt = argc - 1;
        while (1) // 读取一行
        {
            // 从 stdin 中读取 一个 bytes 到 ch 中
            stat = read(0, &ch, 1);
            // stdin 中没有数据，exit
            if (stat == 0)
            {
                exit(0);
            }

            if (ch == ' ' || ch == '\n')
            {
                // 存储前一参数
                buf[buf_cnt++] = 0;
                childArgv[argv_cnt++] = &buf[arg_begin];
                // 更新下一参数起始位置
                arg_begin = buf_cnt;
                // 如果换行 exec
                if (ch == '\n')
                {
                    break;
                }
            }
            else
            {
                buf[buf_cnt++] = ch;
            }
        }

        childArgv[argv_cnt] = 0;
        if (fork() == 0)
        {
            // 执行具体程序
            exec(childArgv[0], childArgv);
        }
        else
        {
            wait(0);
        }
    }
    exit(0);
}