package com.yuanhao;

import java.util.ArrayList;
import java.util.Collection;
import java.util.List;

/**
 * @author Yuanhao
 */
public class Test {
    public static void main(String[] args) {
        new Test().testObjectAndAnyType();
        new Test().testCovariant();
        new Test().testTypeBound();
    }

    /**
     * 此方法用来比较 Java 5 以前采用 Object 超类以及结合接口实现泛型类
     * 和利用 Java 5 泛型特性实现泛型构建的区别
     */
    public void testObjectAndAnyType() {
        // 利用 Object 超类实现泛型
        MyInteger1 myInteger = new MyInteger1();
        MyInteger1 otherInteger = new MyInteger1();
        Shape shape1 = new Shape();
        System.out.println(myInteger.compareTo(otherInteger));
        // 运行时会抛出 ClassCastException
        System.out.println(myInteger.compareTo(shape1));

        // 利用泛型特性实现泛型
        com.yuanhao.MyInteger2 integer2 = new com.yuanhao.MyInteger2();
        com.yuanhao.MyInteger2 otherInteger2 = new com.yuanhao.MyInteger2();
        com.yuanhao.Shape shape2 = new com.yuanhao.Shape();
        System.out.println(integer2.compareTo(otherInteger2));
        // 传递参数时产生一个编译错误
        System.out.println(integer2.compareTo(shape2));
    }

    /**
     * Java 中数组类型是类型兼容的，这叫做协变数组类型 Covariant Array Type
     * 每个数组都明了它所允许存储的对象类型，如果将一个不兼容的类型插入，那么虚拟机抛出 ArrayStoreException
     */
    public void testCovariant() {
        Shape[] arr = new Square[5];
        arr[0] = new Circle();

        // arr = new Square[] {new Circle()}; // 从多态角度

        // Collection<Shape> 如果不加通配符就会产生编译错误
        List<Shape> list1 = new ArrayList<>();
        List<Square> list2 = new ArrayList<>();
        System.out.println(totalArea(list1));
        System.out.println(totalArea(list2));
    }

    public void testTypeBound() {
        // 都可以正常运行
        Shape[] arr1 = new Square[5];
        arr1[0] = new Square();
        arr1[1] = new Square();
        System.out.println(findMax(arr1));

        Square[] arr2 = new Square[5];
        arr2[0] = new Square();
        arr2[1] = new Square();
        System.out.println(findMax(arr2));
    }

    /**
     * 数组协变性
     */
    public static double totalArea(Shape[] arr) {
        double total = 0;
        for (Shape s : arr) {
            total += s.test();
        }
        return total;
    }

    /**
     * Collection<Shape> 如果不加通配符就会产生编译错误
     */
    public static double totalArea(Collection<? extends Shape> arr) {
        double total = 0;
        for (Shape s : arr) {
            total += s.test();
        }
        return total;
    }

    public static <T extends Comparable<? super T>> T findMax(T[] arr) {
        // <T>
        // <T extends Comparable>
        // <T extends Comparable<T>>
        return arr[0].compareTo(arr[1]) == 0 ? arr[0] : arr[1];
    }
}
