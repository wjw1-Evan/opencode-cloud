<template>
  <div>
    <div class="page-head">
      <h2>{{ t('helpTitle') }}</h2>
    </div>

    <div class="card">
      <h3>{{ t('helpIntroTitle') }}</h3>
      <p class="intro-desc">{{ t('helpIntroDesc') }}</p>
      <ul>
        <li><b>{{ t('helpF1') }}</b>：{{ t('helpF1Desc') }}</li>
        <li><b>{{ t('helpF2') }}</b>：{{ t('helpF2Desc') }}</li>
        <li><b>{{ t('helpF3') }}</b>：{{ t('helpF3Desc') }}</li>
        <li><b>{{ t('helpF4') }}</b>：{{ t('helpF4Desc') }}</li>
      </ul>
    </div>

    <div class="card">
      <h3>{{ t('quickStart') }}</h3>
      <ol>
        <li><b>{{ t('step1') }}</b>：{{ t('step1Desc') }}</li>
        <li><b>{{ t('step2') }}</b>：{{ t('step2Desc') }}</li>
        <li><b>{{ t('step3') }}</b>：{{ t('step3Desc') }}</li>
      </ol>
    </div>

    <div class="card">
      <h3>{{ t('usersHelp') }}</h3>
      <ul>
        <li v-html="t('usersHelp1')"></li>
        <li>{{ t('usersHelp2') }}</li>
        <li>{{ t('usersHelp3') }}</li>
        <li>{{ t('usersHelp4') }}</li>
        <li>{{ t('usersHelp5') }}</li>
      </ul>
    </div>

    <div class="card">
      <h3>{{ t('templatesHelp') }}</h3>
      <ul>
        <li v-html="t('templatesHelp1')"></li>
        <li v-html="t('templatesHelp2')"></li>
        <li v-html="t('templatesHelp3')"></li>
        <li>{{ t('templatesHelp4') }}</li>
      </ul>
    </div>

    <div class="card">
      <h3>{{ t('imagesHelp') }}</h3>
      <ul>
        <li v-html="t('imagesHelp1')"></li>
        <li v-html="t('imagesHelp2')"></li>
        <li>{{ t('imagesHelp3') }}</li>
      </ul>
    </div>

    <div class="card">
      <h3>{{ t('accountStatus') }}</h3>
      <ul>
        <li><b>{{ t('enabled') }}</b>：{{ t('accountActive') }}</li>
        <li><b>{{ t('disabled') }}</b>：{{ t('accountDisabled') }}</li>
        <li><b>{{ t('expired') }}</b>：{{ t('accountExpired') }}</li>
      </ul>
    </div>

    <div class="card">
      <h3>{{ t('faq') }}</h3>
      <ul>
        <li v-html="t('faq1')"></li>
        <li v-html="t('faq2')"></li>
        <li v-html="t('faq3')"></li>
        <li v-html="t('faq4')"></li>
        <li v-html="t('faq5')"></li>
      </ul>
    </div>
  </div>
</template>

<script setup>
import { inject } from 'vue'

const { t } = inject('i18n')
</script>

<style scoped>
.page-head { margin-bottom: 20px; }
.card { margin-bottom: 16px; }
ul, ol { margin: 0; padding-left: 20px; line-height: 2; font-size: 14px; color: var(--text-1); }
li b { color: var(--cyan); }
.intro-desc { color: var(--text-1); font-size: 14px; line-height: 1.8; margin: 0 0 10px; }
</style>
