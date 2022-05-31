#include <stdio.h>

int main()
{
    int nums[15] = {};

    int c, nw, state;
    nw = state = 0;

    while ((c = getchar()) != EOF) {
        if (c == ' ' || c == '\t' || c == '\n') {
            if (state) {
                if (nw < 15)
                    nums[nw]++;
                nw = 0;
                state = 0;
            }
        }
        else {
            nw++;
            state = 1;
        }
    }

    for (int i = 0; i < 15; i++) {
        printf("%4d: ", i);
        for (int j = 0; j < nums[i]; j++)
            printf("*");
        printf("\n");
    }
}
