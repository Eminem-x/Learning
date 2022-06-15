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

 Date: 15/06/2022 20:11:01
*/

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- Table structure for sys_roles_resources
-- ----------------------------
DROP TABLE IF EXISTS `sys_roles_resources`;
CREATE TABLE `sys_roles_resources` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `created_at` datetime(3) DEFAULT NULL COMMENT 'created_at',
  `updated_at` datetime(3) DEFAULT NULL COMMENT 'updated_at',
  `deleted_at` datetime(3) DEFAULT NULL COMMENT 'deleted_at',
  `role_id` bigint unsigned NOT NULL COMMENT '角色ID',
  `resource_id` bigint unsigned NOT NULL COMMENT '资源ID',
  `role_type` varchar(255) COLLATE utf8mb4_general_ci DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uniq_roles_with_resources` (`resource_id`,`role_id`),
  KEY `idx_role_id` (`role_id`),
  KEY `idx_resource_id` (`resource_id`),
  KEY `idx_sys_roles_resources_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=175 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='角色资源表';

-- ----------------------------
-- Records of sys_roles_resources
-- ----------------------------
BEGIN;
INSERT INTO `sys_roles_resources` (`id`, `created_at`, `updated_at`, `deleted_at`, `role_id`, `resource_id`, `role_type`) VALUES (1, NULL, NULL, NULL, 0, 0, NULL);
INSERT INTO `sys_roles_resources` (`id`, `created_at`, `updated_at`, `deleted_at`, `role_id`, `resource_id`, `role_type`) VALUES (2, NULL, NULL, NULL, 0, 1, NULL);
INSERT INTO `sys_roles_resources` (`id`, `created_at`, `updated_at`, `deleted_at`, `role_id`, `resource_id`, `role_type`) VALUES (3, NULL, NULL, NULL, 0, 2, NULL);
INSERT INTO `sys_roles_resources` (`id`, `created_at`, `updated_at`, `deleted_at`, `role_id`, `resource_id`, `role_type`) VALUES (16, NULL, NULL, NULL, 2, 3, NULL);
INSERT INTO `sys_roles_resources` (`id`, `created_at`, `updated_at`, `deleted_at`, `role_id`, `resource_id`, `role_type`) VALUES (17, NULL, NULL, NULL, 2, 5, NULL);
INSERT INTO `sys_roles_resources` (`id`, `created_at`, `updated_at`, `deleted_at`, `role_id`, `resource_id`, `role_type`) VALUES (18, NULL, NULL, NULL, 2, 12, NULL);
INSERT INTO `sys_roles_resources` (`id`, `created_at`, `updated_at`, `deleted_at`, `role_id`, `resource_id`, `role_type`) VALUES (19, NULL, NULL, NULL, 2, 13, NULL);
COMMIT;

SET FOREIGN_KEY_CHECKS = 1;
