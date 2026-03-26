# Evaluation Domain Specific Constitution

## I. 命名规范

1. Evaluator 的版本就叫 Evaluator Version , 不能命名为其他任何形式(比如 Evaluator Commit等)
    - 因此跟跟 Evaluator Version 相关模型的命名，都要这样，从 DTO 开始到 Entity 到 DAO 的 Model

## II. 模型规范

1. Evaluator 的数据库表结构存在一定的特殊性，evaluator 表存储评估器的核心元信息，里面没有直接存储评估器的具体内容
    - evaluator_version 表对应 Evaluator 的多个提交版本，里面存放着各个版本的具体内容(LLM评估器的Prompt以及Code评估器的代码内容等，存在metainfo字段内)
        - evaluator 表的 metainfo 字段存储了评估器的元信息，比如评估器的类型(LLM评估器还是Code评估器)，评估器的名称，评估器的描述等
        - 除 id, space_id, evaluator_type, evaluator_id, description, version, base_info 外，其余信息都是存在 metainfo字段内的

## III. 特殊业务逻辑背景输入

### 评测实验相关

> 当前 Evaluation 评测实验的实现逻辑有点儿复杂，这里做一些说明以帮助 LLM 更精确地分析现有代码

1. application 层的 experiment_app.go 中 CreateExperiment 在创建实验后会给 MQ 投递一个实验消息 
   - infra 层 MQ 的 consumer 的 expt_scheduler_event.go 或消费这个消息，并分解实验关联数据集里的一条条数据，继续投递给 MQ 实验数据级别的消息 
     - expt_record_eval.go 则消费实验数据级别的消息，回调评测对象，并将评测对象的输出作为评估器的输入，再回调一下评估器进行打分，最终输出到实验记录中