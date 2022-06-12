#include <stdio.h>

#define LIM 1000

void getLine(char s[]);
void any(char s1[], char s2[]);

int main()
{
    char s1[LIM], s2[LIM];
    getLine(s1);
    getLine(s2);
    any(s1, s2);
}

void getLine(char s[]) {
    int c;
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
}

void any(char s2[], char s1[])
{
    int s[LIM];
    for (int i = 0; s1[i] != '\0'; i++) {
        for (int j = 0; s2[j] != '\0'; j++) {
            if (s1[i] == s2[j]) {
                s[i] = j;
                break;
            }
            if (s2[j + 1] == '\0')
                s[i] = -1;
        }
        if (s1[i + 1] == '\0')
            s[i + 1] = -2;
    }
    for (int i = 0; s[i] != -2; i++)
        printf("%d ", s[i]);
    printf("\n");
}
