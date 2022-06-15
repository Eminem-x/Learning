/*
 Navicat Premium Data Transfer

 Source Server         : mysql
 Source Server Type    : MySQL
 Source Server Version : 80029
 Source Host           : localhost:3306
 Source Schema         : gorm-doc

 Target Server Type    : MySQL
 Target Server Version : 80029
 File Encoding         : 65001

 Date: 15/06/2022 20:11:09
*/

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- Table structure for roles
-- ----------------------------
DROP TABLE IF EXISTS `roles`;
CREATE TABLE `roles` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'id',
  `name` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT '角色名称',
  `description` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '角色描述',
  `created_at` datetime DEFAULT NULL COMMENT 'created_at',
  `updated_at` datetime DEFAULT NULL COMMENT 'updated_at',
  `deleted_at` datetime DEFAULT NULL COMMENT 'deleted_at',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uk_name` (`name`),
  KEY `idx_roles_deleted_at` (`deleted_at`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=95 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='角色表';

-- ----------------------------
-- Records of roles
-- ----------------------------
BEGIN;
INSERT INTO `roles` (`id`, `name`, `description`, `created_at`, `updated_at`, `deleted_at`) VALUES (1, '超级管理员', 'admin', NULL, NULL, NULL);
INSERT INTO `roles` (`id`, `name`, `description`, `created_at`, `updated_at`, `deleted_at`) VALUES (2, '普通管理员', 'test', NULL, NULL, NULL);
INSERT INTO `roles` (`id`, `name`, `description`, `created_at`, `updated_at`, `deleted_at`) VALUES (4, '2-ycx', '2', NULL, '2022-06-13 16:46:56', NULL);
INSERT INTO `roles` (`id`, `name`, `description`, `created_at`, `updated_at`, `deleted_at`) VALUES (6, '4-ycx-yh', '1', NULL, '2022-06-14 11:51:17', NULL);
INSERT INTO `roles` (`id`, `name`, `description`, `created_at`, `updated_at`, `deleted_at`) VALUES (7, '5', NULL, NULL, '2022-06-09 11:32:36', NULL);
INSERT INTO `roles` (`id`, `name`, `description`, `created_at`, `updated_at`, `deleted_at`) VALUES (8, '6', NULL, NULL, '2022-06-09 12:32:42', NULL);
INSERT INTO `roles` (`id`, `name`, `description`, `created_at`, `updated_at`, `deleted_at`) VALUES (9, '2131', '123456123', NULL, '2022-06-10 10:29:17', NULL);
INSERT INTO `roles` (`id`, `name`, `description`, `created_at`, `updated_at`, `deleted_at`) VALUES (93, 'gorm-poly---', 'gorm-poly', '2022-06-15 20:06:25', '2022-06-15 20:06:25', NULL);
COMMIT;

SET FOREIGN_KEY_CHECKS = 1;
