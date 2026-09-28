//go:build windows && cgo

#include <stddef.h>
#include <stdint.h>

struct __emutls_control {
    size_t size;
    size_t align;
    union {
        uintptr_t offset;
        void *ptr;
    };
    void *templ;
};

static void *dummy_once_call = 0;
static void *dummy_once_callable = 0;

struct __emutls_control emutls_once_call __asm__("__emutls_v._ZSt11__once_call") = {
    sizeof(void*), sizeof(void*), {0}, &dummy_once_call
};

struct __emutls_control emutls_once_callable __asm__("__emutls_v._ZSt15__once_callable") = {
    sizeof(void*), sizeof(void*), {0}, &dummy_once_callable
};
