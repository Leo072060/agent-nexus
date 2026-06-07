import type { Address } from '../types'

export async function connectWallet(): Promise<Address> {
  if (!window.ethereum) throw new Error('未检测到浏览器钱包')
  const accounts = await window.ethereum.request<string[]>({ method: 'eth_requestAccounts' })
  const account = accounts[0]
  if (!account) throw new Error('钱包未返回账户')
  return account as Address
}

export async function signMessage(address: Address, message: string): Promise<string> {
  if (!window.ethereum) throw new Error('未检测到浏览器钱包')
  return window.ethereum.request<string>({ method: 'personal_sign', params: [message, address] })
}
