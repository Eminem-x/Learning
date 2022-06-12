#include <stdio.h>

#define LIM 1000

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
    printf("%s\n", s);
}
