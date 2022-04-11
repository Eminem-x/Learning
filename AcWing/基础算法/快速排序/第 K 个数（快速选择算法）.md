### 第 K 个数（快速选择算法）

在序列中选取第 K 小或者大的数，有很多算法，但是时间复杂度大不相同，随着数量级的规模增加，需要更优的算法。

平常刷题过程中，可以偷懒，对 Java 而言，`Arrays.sort()` 或者集合的排序，时间复杂度都是 O(nlogn)，

已经很不错的性能了，但是数量级再大点，就会 <strong>TLE</strong>，因为实际上做了很多无用的排序。

<br>

<strong>基于快速排序的思想，产生了快速选择算法，可以在 O(n) 的时间复杂度下，找到答案。</strong>

关于快速排序，可以参考上一篇文章，那么如何利用快排来达到目的。

<strong>基本思想</strong>：每次只排序需要的那部分，比较 pivot 和 K 来决定排序哪部分。

<strong>代码如下：</strong>随后在原数组中取出第 K 个 值即可

```java
private void quickSort(int[] nums, int k) {
    doSort(nums, 0, nums.length - 1, k);
}

private void doSort(int[] nums, int left, int right, int k) {
    if (left < right) {
        int pivot = partition(nums, left, right);
        // 这一步是关键
        if (k <= pivot) {
            doSort(nums, left, pivot - 1, k);
        } else {
            doSort(nums, pivot, right, k);
        }
    }
}

private int partition(int[] nums, int left, int right) {
    int mid = (left + right) >> 1;
    int pivot = nums[mid];
    
    while (left <= right) {
        while (nums[left] < pivot) {
            left++;
        }
        while (pivot < nums[right]) {
            right--;
        }
        if (left <= right) {
            swap(nums, left, right);
            left++;
            right--;
        }
    }
    return left;
}

private void swap(int[] nums, int idx, int idy) {
    int temp = nums[idx];
    nums[idx] = nums[idy];
    nums[idy] = temp;
}
```

 <br>

<strong>时间复杂度</strong>：

 <img src="https://raw.githubusercontent.com/Eminem-x/Learning/main/AcWing/pic/Part1/选择时间复杂度.png" alt="system call" style="max-width: 70%">

