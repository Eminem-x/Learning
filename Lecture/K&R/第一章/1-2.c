#include <stdio.h>

int main()
{
    // warning: unknown escape sequence '\c' [-Wunknown-escape-sequence]
    printf("Hello\cWorld");
}
