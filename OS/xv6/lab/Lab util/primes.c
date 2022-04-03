#include "kernel/types.h"
#include "kernel/stat.h"
#include "user/user.h"

#define READ 0
#define WRITE 1

// redirect stdin / stdout
void redirect(int fd, int p[])
{
    close(fd);
    dup(p[fd]);
    close(p[READ]);
    close(p[WRITE]);
}

void sievePrimes(int p)
{
    int num;
    while (read(READ, &num, sizeof(num)))
    {
        if (num % p != 0)
        {
            write(WRITE, &num, sizeof(num));
        }
    }
    close(WRITE);
    // very tricky, or the main primes would return after the second process exit.
    wait(0);
    exit(0);
}

void waitNumbers()
{
    int p[2];
    int num;
    if (read(READ, &num, sizeof(num)))
    {
        printf("prime %d\n", num);
        pipe(p);
        if (fork() == 0) // child: recurse
        {
            redirect(READ, p);
            waitNumbers();
        }
        else // parent: prime sieve
        {
            redirect(WRITE, p);
            sievePrimes(num);
        }
    }
}

int main()
{
    int p[2];
    pipe(p);
    if (fork() == 0)
    {
        redirect(READ, p);
        waitNumbers();
    }
    else
    {
        redirect(WRITE, p);
        for (int i = 2; i < 36; i++)
        {
            write(WRITE, &i, sizeof(i));
        }
        close(WRITE);
        wait(0);
        exit(0);
    }
    exit(0);
}