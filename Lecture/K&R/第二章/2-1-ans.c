#include <stdio.h>
#include <limits.h>

int main()
{
    printf("int_limits: %d %d\n", INT_MIN, INT_MAX);
    int n = (unsigned int)~0 >> 1;
    printf("calculate: %d %d\n", -n - 1, n);
}
