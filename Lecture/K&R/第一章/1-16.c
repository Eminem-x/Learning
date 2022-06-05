#include <stdio.h>
#define MAXLINE 65534

int getLine(char line[], int maxline);
void copy(char to[], char form[]);

// terminal 中 command 最大长度是 1024, 因此无法一直输入
// 测试用例存放在 testcase.txt 中
int main()
{
    int len, max;
    char line[MAXLINE];
    char longest[MAXLINE];

    max = 0;
    while ((len = getLine(line, MAXLINE)) > 0 || len == -1)
        if (len > max) {
            max = len;
            copy(longest, line);
        }
    if (max > 0)
        printf("%d %s", max, longest);
    return 0;
}

int getLine(char s[], int lim)
{
    int c, i, flag;
    for (i = 0; (c = getchar()) != EOF && c != '\n'; i++)
        s[i] = c;
    if (c == '\n') {
        s[i] = c;
        i++;
    }
    s[i] = '\0';
    return i;
}

void copy(char to[], char from[])
{
    int i = 0;
    while ((to[i] = from[i]) != '\0')
        i++;
}
