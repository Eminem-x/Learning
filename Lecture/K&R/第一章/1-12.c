#include <stdio.h>

int main()
{
    int c, state;
    state = 0;

    while ((c = getchar()) != EOF) {
        if (c == ' ' || c == '\t' || c == '\n') {
            if (state) {
                putchar('\n');
                state = 0;
            }
        }
        else {
            putchar(c);
            state = 1;
        }
    }
}
