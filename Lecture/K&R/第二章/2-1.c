#include <stdio.h>
#include <limits.h>

// 仅实现 int 的 unsinged 和 signed 取值范围
int main()
{
    int intMax = INT_MAX;
    int intMin = INT_MIN;
    printf("标准库int: %d %d\n", intMax, intMin);

    int uintMax = UINT_MAX;
    printf("标准库uint: %ud\n", uintMax);

    int n = 1;
    while (n > 0)
        n <<= 1;

    printf("计算int: %d %d\n", n - 1, n);

    unsigned int m = 0;
    printf("计算uint: %ud\n", m - 1);
}
