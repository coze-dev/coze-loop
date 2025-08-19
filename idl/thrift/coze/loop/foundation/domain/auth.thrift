namespace go coze.loop.foundation.domain.auth


// 鉴权用户
struct AuthUser {
    1: optional string sso_username                              // 邮箱前缀，与用户ID传一个即可
    2: optional string user_id                                   // 用户ID
}

// 鉴权部门
struct AuthDepartment {
    1: optional string department_id                             // 部门ID
}

// Coze标识
struct AuthCozeIdentifier {
    1: optional string identity_ticket                           // 身份票据
}


// 鉴权主体
struct AuthPrincipal {
    1: optional AuthPrincipalType auth_principal_type           // 主体类型
    2: optional AuthUser auth_user                              // 鉴权用户
    3: optional AuthDepartment auth_department                  // 鉴权部门
    4: optional AuthCozeIdentifier auth_coze_identifier         // Coze标识
}

// 主体类型
enum AuthPrincipalType {
    Undefined = 0
    User = 1                                                    // 用户
    Department = 2                                              // 部门
    CozeIdentifier = 3                                          // 用户身份标识
}

// 鉴权实体类型
typedef string AuthEntityType (ts.enum="true")      // 鉴权实体类型
const AuthEntityType AuthEntityType_Space = "Space" // 空间
const AuthEntityType AuthEntityType_Prompt = "Prompt"
const AuthEntityType AuthEntityType_EvaluationExperiment = "EvaluationExperiment"
const AuthEntityType AuthEntityType_EvaluationSet = "EvaluationSet"
const AuthEntityType AuthEntityType_Evaluator = "Evaluator"
const AuthEntityType AuthEntityType_EvaluationTarget = "EvaluationTarget"
const AuthEntityType AuthEntityType_TraceView = "TraceView"
const AuthEntityType AuthEntityType_Model = "Model"
const AuthEntityType AuthEntityType_Annotation = "Annotation"

// 鉴权资源，客体
struct AuthEntity {
    1: optional string id                                // 实体唯一ID
    2: optional AuthEntityType entity_type               // 实体类型
    3: optional string space_id                          // 空间ID
    4: optional string owner_user_id                     // 实体owner用户ID
}

// 主体+客体+权限点，鉴权组合信息
struct SubjectActionObjects {
    1: optional AuthPrincipal subject                    // 主体，鉴权时通常为用户
    2: optional string action                            // 权限唯一标识
    3: optional list<AuthEntity> objects                 // 客体列表，默认按照或的逻辑处理
}

// 主体+客体+权限点，鉴权结果
struct SubjectActionObjectAuthRes {
    1: optional SubjectActionObjects subject_action_objects // 主体+客体+权限点 鉴权对
    2: optional bool is_allowed                             // 是否允许
}