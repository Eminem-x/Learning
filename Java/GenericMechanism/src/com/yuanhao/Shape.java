package com.yuanhao;

/**
 * @author Yuanhao
 */
public class Shape implements Comparable<Shape> {
    int i;

    @Override
    public int compareTo(Shape o) {
        return this.i - o.i;
    }

    public double test() {
        return 1;
    }
}
