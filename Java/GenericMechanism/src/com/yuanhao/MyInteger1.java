package com.yuanhao;

/**
 * @author Yuanhao
 */
public class MyInteger1 implements Comparable1 {
    int val;

    @Override
    public int compareTo(Object o) {
        MyInteger1 t = (MyInteger1) o;
        return this.val - t.val;
    }
}