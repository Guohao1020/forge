# N3 — Prompt / Skill / AgentSpec 治理(设计草案)

> 切片 3(草案)。隶属 [Nacos 接入总体方案](README.md)。**架构级**,实施前需深化。
> **本切片与既有 Forge 资产重叠最大,深化时务必先厘清"替代 vs 分层"。**

**Goal:** 用 Nacos AI Registry 的 Prompt 模板 / Skill 包(SkillCard)/ AgentSpec 治理能力,给
Forge 的 **Standards(规范中心)+ Skills** 加上版本化、发布/灰度、可见性、多团队共享。

## 1. 缺口 & 重叠

Forge 现有:
- **Standards**(`forge_standard`,F1 themis):workspace→project 两级、category + 画像过滤、
  core 注入 agent instructions / detail 落 skill。DB 内生,**无版本/发布/灰度/跨团队共享**。
- **Skills**(SKILL.md + 文件,agent 绑定):execenv 落盘机制。

Nacos 补:版本(latest/stable/canary)、生命周期(draft→review→published)、namespace 共享、
ecosystem 互通(别的系统也能拉这套规范/技能)。

## 2. Nacos 资源映射

- **Prompt 模板**:带变量的提示模板,版本化 + canary。
- **Skill 包 / SkillCard**:技能定义包(对应 Forge 的 SKILL.md + 文件)。
- **AgentSpec**:agent 规范包(instructions / 行为约束 / 绑定的 skills 清单)。

## 3. Multica/Forge 接入(两种取向,深化时定)

- **取向 X(分层,低风险)**:Forge `forge_standard` / Skills 仍是真相源 + 运行时注入路径不变;
  Nacos 当"版本化发布 + 跨团队分发"的上层,Forge 资产可**发布到 / 拉取自** Nacos。
- **取向 Y(Nacos 治理 + Forge 注入)**:Standards/Skills 的**定义**进 Nacos(版本/发布/灰度),
  F1 `InjectStandards`(claim 时注入)改成**从 Nacos 解析**当前 published/canary 版本 → 注入。
  拿到真治理,但要改 F1 解析路径、与 `forge_standard` schema 对齐迁移。

**倾向**:先做取向 X(发布/分发,不动 F1 注入),验证价值后再评估 Y。

## 4. 关键决策(深化时定 —— 这块最需要 clarifying)

- **替代还是分层** `forge_standard` / Skills?(直接影响 F1 themis 的去留)
- 文件型 Skill(SKILL.md + 资源文件)怎么映射 Nacos skill 包(Nacos 存定义,文件落 execenv?)。
- Prompt/规范的 canary 在"agent 运行"语义下是什么(按 agent / 按 workspace 灰度?)。
- 与 Forge "规范即灵魂"哲学的一致性——治理层别架空规范中心。

## 5. 依赖 & 非目标

- **依赖**:N0;参考 N1 的 resolver/缓存/降级模式。**与 F1(themis)强相关,需联合设计。**
- **非目标**:第一版不做完整评审管线(draft→review→approve 多人审批),先 published/canary;
  不做规范的自动生成/演化(那是另一个方向)。
