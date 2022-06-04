#include <stdio.h>
#define MAXLINE 1000

int getLine(char line[], int maxline);
void reverse(char s[]);

int main()
{
    int len;
    char line[MAXLINE];
    while ((len = getLine(line, MAXLINE)) > 0)
        reverse(line);
    return 0;
}

int getLine(char s[], int maxline)
{
    int c, i;

    for (i = 0; i < maxline - 1 && (c = getchar()) != EOF && c != '\n'; i++)
        s[i] = c;

    if (c == '\n') {
        s[i] = c;
        i++;
    }
    s[i] = '\0';
    return i;
}

void reverse(char s[])
{
    int i = 0;
    while (s[i] != '\0')
        i++;
    int j = i - 1;
    i = 0;
    while (i < j) {
        char c = s[i];
        s[i++] = s[j];
        s[j--] = c;
    }
    i = 1;
    while (s[i] != '\0')
        putchar(s[i++]);
    putchar('\n');
}
