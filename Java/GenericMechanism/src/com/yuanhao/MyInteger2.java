package com.yuanhao;

/**
 * @author Yuanhao
 */
public class MyInteger2 implements Comparable2<MyInteger2> {
    int val;

    @Override
    public int compareTo(MyInteger2 o) {
        return this.val - o.val;
    }
}
