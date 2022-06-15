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

 Date: 15/06/2022 20:11:15
*/

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- Table structure for resources
-- ----------------------------
DROP TABLE IF EXISTS `resources`;
CREATE TABLE `resources` (
  `id` bigint NOT NULL COMMENT 'id',
  `pid` bigint DEFAULT NULL COMMENT '上级菜单ID',
  `title` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '资源标题',
  `type` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '资源类型',
  `component` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '组件',
  `permission` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL COMMENT '权限',
  `hidden` tinyint NOT NULL COMMENT '隐藏',
  `created_at` datetime DEFAULT NULL COMMENT 'created_at',
  `updated_at` datetime DEFAULT NULL COMMENT 'updated_at',
  `deleted_at` datetime DEFAULT NULL COMMENT 'deleted_at',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `idx_title` (`title`),
  KEY `idx_resources_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='资源表';

-- ----------------------------
-- Records of resources
-- ----------------------------
BEGIN;
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (0, NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (1, NULL, '数据看板', '0', NULL, NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (2, NULL, '站点管理', '0', '/service', NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (3, NULL, '物品管理', '0', '/item', NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (4, NULL, '状态监控', '0', NULL, NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (5, NULL, '系统管理', '0', '/system', NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (6, NULL, '操作日志', '0', NULL, NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (11, 3, '库存管理', '1', NULL, NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (12, 3, '物品类型', '1', '/type', NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (13, 3, '物品库', '1', '/warehouse', NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (17, 5, '角色管理', '1', '/role', NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (18, 5, '用户管理', '1', '/user', NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (21, 2, '站点-新增物品', '2', NULL, NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (22, 2, '站点-编辑', '2', NULL, NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (23, 2, '站点-自助柜', '2', NULL, NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (31, 12, '物品类型-增删', '2', NULL, NULL, 0, NULL, NULL, NULL);
INSERT INTO `resources` (`id`, `pid`, `title`, `type`, `component`, `permission`, `hidden`, `created_at`, `updated_at`, `deleted_at`) VALUES (41, 13, '物品库-增删', '2', NULL, NULL, 0, NULL, NULL, NULL);
COMMIT;

SET FOREIGN_KEY_CHECKS = 1;
