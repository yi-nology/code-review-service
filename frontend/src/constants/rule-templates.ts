import type { ReviewRule } from '@/types/config'

/**
 * 预制审查规则模板，按语言/场景分组。
 * 每条规则针对该语言的高频风险点编写，用户一键加入全局规则库。
 * 规则文本是给 AI 审查模型的自然语言指令，措辞保持具体、可执行。
 */
export interface RuleTemplate extends ReviewRule {
  name: string
  category: string
}

export interface RuleTemplateGroup {
  category: string
  label: string
  icon?: string
  templates: RuleTemplate[]
}

export const ruleTemplateGroups: RuleTemplateGroup[] = [
  {
    category: 'common',
    label: '通用',
    templates: [
      {
        name: '通用安全检查',
        category: 'common',
        path: '**/*',
        rule: '重点检查鉴权绕过、越权访问、敏感信息泄露、未校验输入、SQL/命令注入和不安全的默认配置。',
      },
      {
        name: '密钥与凭证',
        category: 'common',
        path: '**/*',
        rule: '检查是否有硬编码的密钥、Token、密码、连接串等凭证；是否使用了不安全的随机数生成；敏感信息是否会被日志或错误响应泄露。',
      },
      {
        name: '错误处理与日志',
        category: 'common',
        path: '**/*',
        rule: '检查异常是否被吞掉、错误是否向上传递、日志是否记录了足够的排查上下文（请求ID/用户ID），以及日志中是否泄露了敏感数据。',
      },
      {
        name: '测试质量',
        category: 'common',
        path: '**/*_test*',
        rule: '重点检查断言是否覆盖关键分支、边界条件、失败路径、并发行为和外部依赖隔离。',
      },
    ],
  },
  {
    category: 'rust',
    label: 'Rust',
    templates: [
      {
        name: 'Rust 内存与所有权',
        category: 'rust',
        path: '**/*.rs',
        rule: '重点检查 unsafe 块的使用是否必要且安全、裸指针解引用、手动内存管理错误；检查所有权与借用是否会导致悬垂引用或数据竞争；避免不必要的 clone 影响性能。',
      },
      {
        name: 'Rust 并发安全',
        category: 'rust',
        path: '**/*.rs',
        rule: '检查 Send/Sync 约束是否正确、锁的获取顺序是否可能死锁、是否存在跨线程的数据竞争、async 任务是否正确处理取消与超时。',
      },
      {
        name: 'Rust 错误处理',
        category: 'rust',
        path: '**/*.rs',
        rule: '避免用 unwrap()/expect() 处理可能失败的操作（尤其生产代码）；检查 ? 操作符的错误类型是否兼容、Result/Option 是否被正确传播、是否有错误被静默忽略。',
      },
    ],
  },
  {
    category: 'c_cpp',
    label: 'C / C++',
    templates: [
      {
        name: 'C/C++ 内存安全',
        category: 'c_cpp',
        path: '**/*.{c,h,cpp,cc,hpp,cxx}',
        rule: '重点检查缓冲区溢出、越界访问、use-after-free、双重释放、空指针解引用、未初始化变量使用；检查数组/指针边界、malloc/new 后是否配对 free/delete。',
      },
      {
        name: 'C/C++ 资源管理',
        category: 'c_cpp',
        path: '**/*.{c,h,cpp,cc,hpp,cxx}',
        rule: '检查所有资源（内存、文件句柄、socket、锁）在所有退出路径（含错误返回）上是否正确释放；优先使用 RAII、智能指针、goto cleanup 或 defer 模式。',
      },
      {
        name: 'C/C++ 不安全函数',
        category: 'c_cpp',
        path: '**/*.{c,h,cpp,cc,hpp,cxx}',
        rule: '检查是否使用了不安全的标准函数（strcpy/strcat/sprintf/gets 等），应改用带长度限制的版本（strncpy/snprintf/fgets 等）；检查整数溢出导致的长度绕过。',
      },
    ],
  },
  {
    category: 'go',
    label: 'Go',
    templates: [
      {
        name: 'Go 并发与 goroutine',
        category: 'go',
        path: '**/*.go',
        rule: '检查 goroutine 泄漏（启动后无退出条件）、channel 是否可能死锁或永久阻塞、是否有未同步的共享变量访问（应用 mutex 或 atomic）、context 是否正确传递与取消。',
      },
      {
        name: 'Go 错误处理',
        category: 'go',
        path: '**/*.go',
        rule: '检查 error 是否被忽略（如 `result, _ := ...`）、是否应该 wrapping 错误（fmt.Errorf %w）、panic 是否在非初始化路径被滥用、defer 中的 error 是否被记录。',
      },
      {
        name: 'Go 资源与性能',
        category: 'go',
        path: '**/*.go',
        rule: '检查 http.Response.Body/文件/sql.Rows 是否正确 Close（defer Close）、rows 是否在循环中正确迭代、是否有 N+1 查询、sync.Mutex 是否可能被 Copy。',
      },
    ],
  },
  {
    category: 'node',
    label: 'Node.js',
    templates: [
      {
        name: 'Node.js 异步与错误',
        category: 'node',
        path: '**/*.{js,mjs,cjs}',
        rule: '检查未 await 的 Promise（fire-and-forget 导致未捕获异常）、async 函数中的错误是否被 try/catch 覆盖、是否有回调与 Promise 混用、事件监听器是否泄漏。',
      },
      {
        name: 'Node.js 安全',
        category: 'node',
        path: '**/*.{js,mjs,cjs}',
        rule: '检查是否有原型链污染风险（Object.assign/merge 用户输入）、eval/Function 构造器执行动态代码、require 动态路径、正则 ReDoS、依赖的已知漏洞。',
      },
    ],
  },
  {
    category: 'typescript',
    label: 'TypeScript',
    templates: [
      {
        name: 'TypeScript 类型安全',
        category: 'typescript',
        path: '**/*.{ts,tsx}',
        rule: '检查 any 的使用是否可避免（应改用 unknown/泛型/具体类型）、as 类型断言是否掩盖了真实错误、是否有 @ts-ignore 或非空断言 ! 滥用、类型守卫是否完备。',
      },
      {
        name: 'TypeScript 前端通用',
        category: 'typescript',
        path: '**/*.{ts,tsx}',
        rule: '重点检查接口错误处理、加载/空状态、权限态、表单校验、响应式布局和用户操作后的反馈；检查 Promise 是否有 await/catch。',
      },
    ],
  },
  {
    category: 'vue',
    label: 'Vue',
    templates: [
      {
        name: 'Vue 响应式与内存',
        category: 'vue',
        path: '**/*.vue',
        rule: '检查响应式数据是否正确声明（ref/reactive/computed）、watch/watchEffect 是否在组件卸载时清理（定时器、事件监听、WebSocket）、是否有不必要的深度监听导致性能问题。',
      },
      {
        name: 'Vue 组件设计',
        category: 'vue',
        path: '**/*.vue',
        rule: '检查 props 类型与默认值是否定义、v-for 是否有稳定的 key、v-html 是否有 XSS 风险、组件是否过大应拆分、是否有命名冲突的 computed/data。',
      },
    ],
  },
  {
    category: 'react',
    label: 'React',
    templates: [
      {
        name: 'React Hooks',
        category: 'react',
        path: '**/*.{jsx,tsx}',
        rule: '检查 useEffect 依赖数组是否完整且正确（缺失依赖导致 stale closure、多余依赖导致无限重渲染）、是否在 effect 中正确清理副作用、是否有条件调用 Hook。',
      },
      {
        name: 'React 渲染与性能',
        category: 'react',
        path: '**/*.{jsx,tsx}',
        rule: '检查是否有不必要的重渲染（缺 memo/useMemo/useCallback）、key 是否用数组 index（在增删时会导致 bug）、state 更新是否依赖前值（应用函数式更新）、是否有直接修改 state 的代码。',
      },
    ],
  },
  {
    category: 'bash',
    label: 'Bash / Shell',
    templates: [
      {
        name: 'Shell 脚本健壮性',
        category: 'bash',
        path: '**/*.{sh,bash,zsh}',
        rule: '检查脚本是否以 set -euo pipefail 开头（未定义变量/管道失败/命令失败应中止）、变量是否加引号防止分词与通配、是否有未检查退出码的关键命令、cd 后是否处理失败。',
      },
      {
        name: 'Shell 安全',
        category: 'bash',
        path: '**/*.{sh,bash,zsh}',
        rule: '检查是否有命令注入风险（eval/$(... 拼接用户输入）、临时文件是否用了可预测的名称（应 mktemp）、是否 chmod 敏感文件、是否有硬编码路径假设。',
      },
    ],
  },
  {
    category: 'python',
    label: 'Python',
    templates: [
      {
        name: 'Python 安全',
        category: 'python',
        path: '**/*.{py,pyw}',
        rule: '检查是否有 eval/exec/pickle 处理不可信输入、SQL 是否用参数化查询（非字符串拼接）、subsystem shell 是否注入风险、是否有不安全的 yaml.load（应用 safe_load）。',
      },
      {
        name: 'Python 异步与异常',
        category: 'python',
        path: '**/*.{py,pyw}',
        rule: '检查 async/await 是否正确使用（阻塞调用在 async 函数中、任务是否 gather 时异常被吞）、bare except 是否掩盖真实错误、资源是否用 with 上下文管理。',
      },
    ],
  },
  {
    category: 'java',
    label: 'Java',
    templates: [
      {
        name: 'Java 安全',
        category: 'java',
        path: '**/*.{java}',
        rule: '检查是否有 SQL 注入（Statement 拼接 vs PreparedStatement）、反序列化不可信数据、XML 外部实体（XXE）、不安全的反射、权限校验是否绕过。',
      },
      {
        name: 'Java 并发',
        category: 'java',
        path: '**/*.{java}',
        rule: '检查共享可变状态是否有同步（synchronized/Lock/Concurrent 集合）、是否可能死锁、ThreadLocal 是否泄漏、Future 是否处理超时与中断。',
      },
    ],
  },
  {
    category: 'infra',
    label: '配置与基础设施',
    templates: [
      {
        name: '数据库变更',
        category: 'infra',
        path: '**/*.{sql}',
        rule: '重点检查迁移兼容性（是否有破坏性 DDL 如 DROP/RENAME）、索引缺失、大表 ALTER 是否锁表、是否需要 backfill、是否有不可逆操作。',
      },
      {
        name: 'Docker / CI 配置',
        category: 'infra',
        path: '**/{Dockerfile,*.dockerfile,.github/workflows/*,*.yml,*.yaml}',
        rule: '检查镜像是否用了最小基础镜像、是否泄露了凭证（ENV 中硬编码密钥）、CI 是否在 PR 上运行了必要检查、是否有 least-privilege 的 token 权限。',
      },
    ],
  },
]

/** 扁平化所有模板，兼容旧的扁平数组用法。 */
export const ruleTemplates: RuleTemplate[] = ruleTemplateGroups.flatMap((g) => g.templates)
