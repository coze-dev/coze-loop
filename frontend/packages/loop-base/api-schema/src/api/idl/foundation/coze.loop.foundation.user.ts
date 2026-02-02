// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as user from './domain/user';
export { user };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface UserRegisterRequest {
  email?: string,
  password?: string,
}
export interface UserRegisterResponse {
  user_info?: user.UserInfoDetail,
  token?: string,
  expire_time?: number,
}
export interface LoginByPasswordRequest {
  email?: string,
  password?: string,
}
export interface LoginByPasswordResponse {
  user_info?: user.UserInfoDetail,
  token?: string,
  expire_time?: number,
}
export interface LogoutRequest {
  token?: string
}
export interface LogoutResponse {}
export interface ResetPasswordRequest {
  email?: string,
  password?: string,
  code?: string,
}
export interface ResetPasswordResponse {}
export interface GetUserInfoByTokenRequest {
  token?: string
}
export interface GetUserInfoByTokenResponse {
  user_info?: user.UserInfoDetail
}
export interface ModifyUserProfileRequest {
  user_id?: string,
  /** 用户唯一名称 */
  name?: string,
  /** 用户昵称 */
  nick_name?: string,
  /** 用户描述 */
  description?: string,
  /** 用户头像URI */
  avatar_uri?: string,
}
export interface ModifyUserProfileResponse {
  user_info?: user.UserInfoDetail
}
export interface GetUserInfoRequest {
  user_id?: string
}
export interface GetUserInfoResponse {
  user_info?: user.UserInfoDetail
}
export interface MGetUserInfoRequest {
  user_ids?: string[]
}
export interface MGetUserInfoResponse {
  user_infos?: user.UserInfoDetail[]
}
/** 用户注册相关接口 */
export const Register = /*#__PURE__*/createAPI<UserRegisterRequest, UserRegisterResponse>({
  "url": "/api/foundation/v1/users/register",
  "method": "POST",
  "name": "Register",
  "reqType": "UserRegisterRequest",
  "reqMapping": {
    "body": ["email", "password"]
  },
  "resType": "UserRegisterResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.user",
  "service": "foundationUser"
});
export const ResetPassword = /*#__PURE__*/createAPI<ResetPasswordRequest, ResetPasswordResponse>({
  "url": "/api/foundation/v1/users/reset_password",
  "method": "POST",
  "name": "ResetPassword",
  "reqType": "ResetPasswordRequest",
  "reqMapping": {
    "body": ["email", "password", "code"]
  },
  "resType": "ResetPasswordResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.user",
  "service": "foundationUser"
});
/** 用户登陆相关接口 */
export const LoginByPassword = /*#__PURE__*/createAPI<LoginByPasswordRequest, LoginByPasswordResponse>({
  "url": "/api/foundation/v1/users/login_by_password",
  "method": "POST",
  "name": "LoginByPassword",
  "reqType": "LoginByPasswordRequest",
  "reqMapping": {
    "body": ["email", "password"]
  },
  "resType": "LoginByPasswordResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.user",
  "service": "foundationUser"
});
export const Logout = /*#__PURE__*/createAPI<LogoutRequest, LogoutResponse>({
  "url": "/api/foundation/v1/users/logout",
  "method": "POST",
  "name": "Logout",
  "reqType": "LogoutRequest",
  "reqMapping": {
    "body": ["token"]
  },
  "resType": "LogoutResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.user",
  "service": "foundationUser"
});
/** 修改用户资料相关接口 */
export const ModifyUserProfile = /*#__PURE__*/createAPI<ModifyUserProfileRequest, ModifyUserProfileResponse>({
  "url": "/api/foundation/v1/users/:user_id/update_profile",
  "method": "PUT",
  "name": "ModifyUserProfile",
  "reqType": "ModifyUserProfileRequest",
  "reqMapping": {
    "path": ["user_id"],
    "body": ["name", "nick_name", "description", "avatar_uri"]
  },
  "resType": "ModifyUserProfileResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.user",
  "service": "foundationUser"
});
/** 基于登陆态获取用户信息相关接口 */
export const GetUserInfoByToken = /*#__PURE__*/createAPI<GetUserInfoByTokenRequest, GetUserInfoByTokenResponse>({
  "url": "/api/foundation/v1/users/session",
  "method": "GET",
  "name": "GetUserInfoByToken",
  "reqType": "GetUserInfoByTokenRequest",
  "reqMapping": {
    "query": ["token"]
  },
  "resType": "GetUserInfoByTokenResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.user",
  "service": "foundationUser"
});