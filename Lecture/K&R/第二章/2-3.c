#include <stdio.h>

#define LIM 1000

void htoi(char s[]);

// 在 2-2 的基础上添加 htoi 函数
int main()
{
    int c;
    char s[LIM];
    for (int i = 0; i < LIM - 1; i++) {
        if ((c = getchar()) != '\n') {
            if (c != EOF) {
                s[i] = c;
            } else {
                s[i] = '\0';
                break;
            }
        } else {
            s[i] = '\0';
            break;
        }
    }
    s[LIM - 1] = '\0';
    htoi(s);
}

void htoi(char s[])
{
    int n = 0;
    int i = 0;
    // get the length of s and lower s
    for (i = 0; s[i] != '\0'; i++) {
        if (i == 0) {
            if (s[i] != '0') {
                printf("error type!\n");
                return;
            }
        } else if (i == 1) {
            if (s[i] != 'x' && s[i] != 'X') {
                printf("error type!\n");
                return;
            }
        } else {
            if (s[i] >= 'A' && s[i] <= 'F')
                s[i] -= 'A' + 'a';
        }
    }

    // calculate
    for (int j = 2; j < i; j++) {
        if (s[j] >= '0' && s[j] <= '9')
            n = n * 16 + s[j] - '0';
        else
            n = n * 16 + s[j] - 'a' + 10;
    }
    printf("%d\n", n);
}
