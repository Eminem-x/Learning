#include <stdio.h>

int main()
{
    int c;
    int flag = 0;
    while ((c = getchar()) != EOF)
    {
        if (c == ' ')
            flag = 1;
        if (!flag)
            printf("%c", c);
        if (flag && c != ' ')
        {
            printf(" %c", c);
            flag = 0;
        }
    }
}
