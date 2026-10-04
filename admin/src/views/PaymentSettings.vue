<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { errorMessage, type AdminSchemas } from '@xzji/api-client/admin'
import { api } from '../api'

type Input = AdminSchemas['PaymentSettingsInput']

const loading = ref(true)
const saving = ref(false)
const view = ref<AdminSchemas['PaymentSettings'] | null>(null)
// 密钥字段不回显，留空表示不修改
const form = reactive<Input>({
  notifyBaseUrl: '', siteUrl: '', balance: true,
  alipay: { enabled: false, sandbox: false, appId: '', keyMode: 'key', privateKey: '', alipayPublicKey: '', appCert: '', rootCert: '', alipayCert: '' },
  wechat: { enabled: false, h5: false, appId: '', mchId: '', serialNo: '', apiV3Key: '', privateKey: '', publicKeyId: '', publicKey: '' },
})

function fill(v: AdminSchemas['PaymentSettings']) {
  view.value = v
  const { alipay: a, wechat: w } = v
  Object.assign(form, { notifyBaseUrl: v.notifyBaseUrl, siteUrl: v.siteUrl, balance: v.balance })
  Object.assign(form.alipay, {
    enabled: a.enabled, sandbox: a.sandbox, appId: a.appId, keyMode: a.keyMode, privateKey: '',
    alipayPublicKey: a.alipayPublicKey, appCert: a.appCert, rootCert: a.rootCert, alipayCert: a.alipayCert,
  })
  Object.assign(form.wechat, {
    enabled: w.enabled, h5: w.h5, appId: w.appId, mchId: w.mchId, serialNo: w.serialNo, apiV3Key: '', privateKey: '',
    publicKeyId: w.publicKeyId, publicKey: w.publicKey,
  })
}

async function load() {
  loading.value = true
  const { data, error } = await api.GET('/settings/payment')
  loading.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  fill(data)
}

async function save() {
  saving.value = true
  const { data, error } = await api.PUT('/settings/payment', { body: form })
  saving.value = false
  if (!data) return ElMessage.error(errorMessage(error))
  fill(data)
  ElMessage.success('已保存')
}

onMounted(load)
</script>

<template>
  <el-card v-loading="loading" shadow="never">
    <el-alert v-if="view?.mock" type="warning" :closable="false" show-icon class="gap-b" title="后端正以模拟支付模式运行" description="微信、支付宝都不连真实渠道，前台付款时会出现“模拟支付成功”按钮，只能用于开发环境。在 config.yaml 里把 pay.mock 改为 false 并重启后，下面的设置才会生效。" />
    <el-form label-width="130px" class="form" @submit.prevent="save">
      <h3>基本</h3>
      <el-form-item label="接口地址">
        <el-input v-model="form.notifyBaseUrl" placeholder="https://api.example.com" />
        <p class="hint">接口的对外地址，支付渠道把付款结果通知到这里，必须能从公网访问。</p>
      </el-form-item>
      <el-form-item label="前台地址">
        <el-input v-model="form.siteUrl" placeholder="https://www.example.com" />
        <p class="hint">付款完成后跳回 {{ form.siteUrl || '前台地址' }}/pay/result 确认结果。</p>
      </el-form-item>
      <el-form-item label="余额支付">
        <el-switch v-model="form.balance" active-text="允许用余额购买内容和开通会员" />
      </el-form-item>

      <h3>支付宝</h3>
      <el-form-item label="启用">
        <el-switch v-model="form.alipay.enabled" active-text="电脑网站支付 + 手机网站支付" />
      </el-form-item>
      <el-form-item label="环境">
        <el-radio-group v-model="form.alipay.sandbox">
          <el-radio :value="false">正式环境</el-radio>
          <el-radio :value="true">沙箱环境</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item label="AppID">
        <el-input v-model="form.alipay.appId" />
      </el-form-item>
      <el-form-item label="应用私钥">
        <el-input v-model="form.alipay.privateKey" type="textarea" :rows="3" :placeholder="view?.alipay.hasPrivateKey ? '已填写，留空表示不修改' : '支付宝开放平台密钥工具生成的应用私钥'" />
      </el-form-item>
      <el-form-item label="验签方式">
        <el-radio-group v-model="form.alipay.keyMode">
          <el-radio value="key">支付宝公钥</el-radio>
          <el-radio value="cert">公钥证书</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item v-if="form.alipay.keyMode === 'key'" label="支付宝公钥">
        <el-input v-model="form.alipay.alipayPublicKey" type="textarea" :rows="3" placeholder="开放平台“接口加签方式”里查看的支付宝公钥（不是应用公钥）" />
      </el-form-item>
      <template v-else>
        <el-form-item label="应用公钥证书">
          <el-input v-model="form.alipay.appCert" type="textarea" :rows="3" placeholder="appCertPublicKey_xxx.crt 的内容" />
        </el-form-item>
        <el-form-item label="支付宝根证书">
          <el-input v-model="form.alipay.rootCert" type="textarea" :rows="3" placeholder="alipayRootCert.crt 的内容" />
        </el-form-item>
        <el-form-item label="支付宝公钥证书">
          <el-input v-model="form.alipay.alipayCert" type="textarea" :rows="3" placeholder="alipayCertPublicKey_RSA2.crt 的内容" />
        </el-form-item>
      </template>
      <el-form-item v-if="view?.notifyUrls.alipay" label="回调地址">
        <code>{{ view.notifyUrls.alipay }}</code>
      </el-form-item>

      <h3>微信支付</h3>
      <el-form-item label="启用">
        <el-switch v-model="form.wechat.enabled" active-text="Native 扫码支付（电脑）" />
      </el-form-item>
      <el-form-item label="H5 支付">
        <el-switch v-model="form.wechat.h5" active-text="已开通 H5 支付，手机浏览器可用" />
      </el-form-item>
      <el-form-item label="AppID">
        <el-input v-model="form.wechat.appId" placeholder="与商户号绑定的公众号或网站应用 AppID" />
      </el-form-item>
      <el-form-item label="商户号">
        <el-input v-model="form.wechat.mchId" />
      </el-form-item>
      <el-form-item label="证书序列号">
        <el-input v-model="form.wechat.serialNo" placeholder="商户 API 证书的序列号" />
      </el-form-item>
      <el-form-item label="APIv3 密钥">
        <el-input v-model="form.wechat.apiV3Key" type="password" show-password :placeholder="view?.wechat.hasApiV3Key ? '已填写，留空表示不修改' : '商户平台设置的 32 位 APIv3 密钥'" />
      </el-form-item>
      <el-form-item label="商户私钥">
        <el-input v-model="form.wechat.privateKey" type="textarea" :rows="3" :placeholder="view?.wechat.hasPrivateKey ? '已填写，留空表示不修改' : 'apiclient_key.pem 的内容'" />
      </el-form-item>
      <el-form-item label="微信支付公钥 ID">
        <el-input v-model="form.wechat.publicKeyId" placeholder="PUB_KEY_ID_ 开头；留空时自动下载平台证书验签" />
      </el-form-item>
      <el-form-item label="微信支付公钥">
        <el-input v-model="form.wechat.publicKey" type="textarea" :rows="3" placeholder="pub_key.pem 的内容" />
      </el-form-item>
      <el-form-item v-if="view?.notifyUrls.wechat" label="回调地址">
        <code>{{ view.notifyUrls.wechat }}</code>
      </el-form-item>

      <el-form-item>
        <el-button type="primary" native-type="submit" :loading="saving">保存</el-button>
        <span class="hint inline">保存时会用填写的参数实际创建一次支付客户端，参数有误会提示。私钥和 APIv3 密钥加密保存。</span>
      </el-form-item>
    </el-form>
  </el-card>
</template>

<style scoped>
.form { max-width: 760px; }
.form h3 { margin: 22px 0 14px; padding-left: 10px; border-left: 3px solid #2F6BFF; font-size: 15px; color: #262A31; }
.form h3:first-child { margin-top: 0; }
.hint { margin: 4px 0 0; width: 100%; font-size: 12px; line-height: 1.6; color: #8A9099; }
.hint.inline { width: auto; margin: 0 0 0 12px; }
.gap-b { margin-bottom: 18px; }
code { font-size: 13px; color: #262A31; background: #F6F7F9; padding: 2px 8px; border-radius: 4px; }
</style>
