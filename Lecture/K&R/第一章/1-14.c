#include <stdio.h>

// 答案实现的更加全面 数组大小定义为 128
// 并且调用了 isprint 函数 判断是否可显
int main()
{
    int c;
    int nums[30] = {};

    while ((c = getchar()) != EOF) {
        if (c >= 'a' && c <= 'z')
            nums[c - 'a']++;
        else if (c == ' ')
            nums[26]++;
        else if (c == '\t')
            nums[27]++;
        else if (c == '\n')
            nums[28]++;
    }
    for (int i = 0; i < 29; i++) {
        if (i < 26)
            printf(" %c: ", 'a' + i);
        else if (i == 26)
            printf(" :  ");
        else if (i == 27)
            printf("\\t: ");
        else if (i == 28)
            printf("\\n: ");

        for (int j = 0; j < nums[i]; j++)
            printf("*");
        printf("\n");
    }
}
