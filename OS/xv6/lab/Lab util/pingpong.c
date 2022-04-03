#include "kernel/types.h"
#include "kernel/stat.h"
#include "user/user.h"

int main()
{
    int p1[2], p2[2];
    pipe(p1);
    pipe(p2);
    if (fork() == 0)
    {
        // maybe different from wc system call
        // close(p1[1]);
        // close(p2[0]);
        char arr[] = {'Y'};
        long length = sizeof(arr);
        read(p1[0], arr, length);
        printf("%d: received ping %c\n", getpid(), arr[0]);
        write(p2[1], arr, length);
        exit(0);
    }
    else
    {
        char arr[] = {'X'};
        long length = sizeof(arr);
        // close(p1[0]);
        // close(p2[1]);
        write(p1[1], arr, length);
        wait((int *)0);
        read(p2[0], arr, length);
        printf("%d: received pong %c\n", getpid(), arr[0]);
        exit(0);
    }
}