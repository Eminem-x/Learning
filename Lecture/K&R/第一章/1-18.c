#include <stdio.h>
#define MAXLINE 65534

int getLine(char line[], int maxline);
void trim(char to[], char form[]);

// terminal 中 command 最大长度是 1024, 因此无法一直输入
// 测试用例存放在 testcase.txt 中
int main()
{
    int len;
    char line[MAXLINE];
    char s[MAXLINE];

    while ((len = getLine(line, MAXLINE)) > 0 || len == -1) {
        trim(s, line);
        printf("%s", s);
    }
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

void trim(char to[], char from[])
{
    int i = 0;
    while (from[i] != '\0')
        i++;
    int j = i - 2;
    int flag = 1;
    while (j >= 0) {
        if ((from[j] != '\t' && from[j] != ' ') || !flag) {
            if (flag) {
                flag = 0;
                to[j + 1] = '!';
                to[j + 2] = '\n';
                to[j + 3] = '\0';
            }
            to[j] = from[j];
        }
        j--;
    }
}
