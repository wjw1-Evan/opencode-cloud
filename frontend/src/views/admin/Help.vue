<template>
  <div>
    <div class="page-head">
      <h2>使用帮助</h2>
    </div>

    <div class="card">
      <h3>三步开课</h3>
      <ol>
        <li><b>建模板</b>：进入「镜像模板」，填入工具镜像与主端口（如 opencode 4096、code-server 8080、JupyterLab 8888），可按需添加附加端口、环境变量与资源限额。</li>
        <li><b>批量建号</b>：进入「用户与容器」，填写课程、前缀与数量，选择模板后点击「生成账号并建容器」。系统自动生成用户名与随机密码，并立即为每人创建独立容器，完成后浏览器会下载 accounts.csv（含全部账号密码），可直接分发给用户。</li>
        <li><b>用户登录</b>：用户打开门户页面，用账号密码登录后自动进入自己专属的工具界面，无需安装任何软件。</li>
      </ol>
    </div>

    <div class="card">
      <h3>用户与容器</h3>
      <ul>
        <li>列表可按<b>课程</b>筛选，勾选用户后批量执行<b>重建 / 启动 / 重启 / 停止 / 删除</b>。</li>
        <li>「重建」会删除并重建选中用户的容器（数据卷保留），已有容器不重建。</li>
        <li>密码由用户自助修改，管理员可在列表中查看当前密码。</li>
        <li>容器空闲 30 分钟自动停止以节省资源，用户再次访问时自动秒级唤醒。</li>
        <li>一个用户对应一个容器，删除用户即移除其容器。</li>
      </ul>
    </div>

    <div class="card">
      <h3>镜像模板</h3>
      <ul>
        <li>系统内置 <b>vscode</b> 模板（code-server，端口 8080），可直接选用；也可自行创建模板，填写任意提供 Web 界面的 Docker 镜像（如 opencode、Dify、JupyterLab…）。</li>
        <li>用户登录后访问站点根路径即进入自己的工具；<b>附加端口</b>通过 <code>/port/{端口}/</code> 访问。</li>
        <li>示例：opencode 使用启动命令 <code>serve --mdns</code>（主端口 4096）。</li>
        <li>同一课程的用户使用同一模板；不同课程可使用不同模板，互不影响。</li>
      </ul>
    </div>

    <div class="card">
      <h3>账号状态</h3>
      <ul>
        <li><b>active</b>：正常使用中。</li>
        <li><b>disabled</b>：管理员禁用，无法登录。</li>
        <li><b>expired</b>：已到期，到期账号自动置为过期并停止容器。</li>
      </ul>
    </div>

    <div class="card">
      <h3>常见问题</h3>
      <ul>
        <li><b>用户打不开工具？</b>检查该用户容器状态是否为 running，必要时在「用户与容器」中重启或重建。</li>
        <li><b>端口访问不了？</b>确认模板中已配置对应端口，且访问路径使用 <code>/port/{端口}/</code>。</li>
        <li><b>忘记用户密码？</b>管理员可在用户列表查看当前密码；用户也可在门户页自行修改。</li>
        <li><b>代码会丢吗？</b>不会。每个用户的代码保存在专属数据卷中，容器重启不丢失。</li>
      </ul>
    </div>
  </div>
</template>

<script setup>
</script>

<style scoped>
.page-head { margin-bottom: 20px; }
h2 { margin: 0; font-size: 20px; letter-spacing: 0.02em; }
.card { margin-bottom: 16px; }
h3 { margin: 0 0 12px; font-size: 14.5px; color: var(--text-0); letter-spacing: 0.03em; }
ul, ol { margin: 0; padding-left: 20px; line-height: 2; font-size: 14px; color: var(--text-1); }
li b { color: var(--cyan); }
</style>
