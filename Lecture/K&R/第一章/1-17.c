#include <stdio.h>
#define MAXLINE 1000

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
    while ((len = getLine(line, MAXLINE)) > 0)
        if (len > 80)
            printf("%s", line);
    if (max > 0)
        printf("%d %s", max, longest);
    return 0;
}

int getLine(char s[], int lim)
{
    int c, i, flag;
    for (i = 0; i < lim - 1 &&  (c = getchar()) != EOF && c != '\n'; i++)
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
