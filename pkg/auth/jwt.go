package auth

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"bs-net-monitor/internal/conf"
)

var (
	rsaPublicKey     *rsa.PublicKey
	rsaPublicKeyOnce sync.Once
	rsaPublicKeyErr  error
)

// getRSAPublicKey 懒加载并缓存 RSA 公钥。
func getRSAPublicKey() (*rsa.PublicKey, error) {
	rsaPublicKeyOnce.Do(func() {
		pemStr := conf.GetConfig().Auth.RSAPublicKey
		if pemStr == "" {
			rsaPublicKeyErr = errors.New("未配置 RSA 公钥")
			return
		}
		rsaPublicKey, rsaPublicKeyErr = parseRSAPublicKey(pemStr)
	})
	return rsaPublicKey, rsaPublicKeyErr
}

// parseRSAPublicKey 解析 RSA 公钥。
// 支持两种格式：
//  1. 完整 PEM 格式（含 -----BEGIN PUBLIC KEY----- 头尾）
//  2. 裸 Base64 字符串（不含头尾，直接复制公钥内容）
func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	pemStr = strings.TrimSpace(pemStr)

	// 如果不含 PEM 头，则将裸 base64 包装成标准 PEM
	if !strings.Contains(pemStr, "-----") {
		// 移除所有空白换行，重新按 64 字符分行
		clean := strings.Join(strings.Fields(pemStr), "")
		const lineLen = 64
		var lines []string
		for i := 0; i < len(clean); i += lineLen {
			end := i + lineLen
			if end > len(clean) {
				end = len(clean)
			}
			lines = append(lines, clean[i:end])
		}
		pemStr = "-----BEGIN PUBLIC KEY-----\n" +
			strings.Join(lines, "\n") +
			"\n-----END PUBLIC KEY-----"
	}

	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("RSA 公钥 PEM 格式错误，无法解码")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// 尝试 PKCS1 格式
		rsaPub, err2 := x509.ParsePKCS1PublicKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("解析 RSA 公钥失败 (PKIX: %v, PKCS1: %v)", err, err2)
		}
		return rsaPub, nil
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("公钥不是 RSA 公钥")
	}
	return rsaPub, nil
}

// parseBase64URLInt base64url 解码大整数（JWT 中 n 的编码方式）。
func parseBase64URLInt(s string) (*big.Int, error) {
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(data), nil
}

// VerifyJWT 使用配置中的 RSA 公钥校验 JWT 签名和过期时间。
// 返回解析后的 claims。
func VerifyJWT(token string) (map[string]any, error) {
	pub, err := getRSAPublicKey()
	if err != nil {
		log.Printf("[JWT] 获取 RSA 公钥失败: %v", err)
		return nil, err
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		log.Printf("[JWT] Token 格式错误，分段数: %d", len(parts))
		return nil, errors.New("JWT 格式错误")
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		log.Printf("[JWT] 解码 header 失败: %v", err)
		return nil, fmt.Errorf("解码 JWT header 失败: %w", err)
	}
	var header map[string]any
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		log.Printf("[JWT] 解析 header 失败: %v", err)
		return nil, fmt.Errorf("解析 JWT header 失败: %w", err)
	}
	if alg, _ := header["alg"].(string); alg != "RS256" {
		log.Printf("[JWT] 不支持的算法: %s", alg)
		return nil, fmt.Errorf("不支持的 JWT 算法: %s", alg)
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		log.Printf("[JWT] 解码 payload 失败: %v", err)
		return nil, fmt.Errorf("解码 JWT payload 失败: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		log.Printf("[JWT] 解析 payload 失败: %v", err)
		return nil, fmt.Errorf("解析 JWT payload 失败: %w", err)
	}

	// 校验 exp（仅当 JWT 中存在 exp 字段时才校验，允许不含 exp 的永久 token）
	// exp 支持 Unix 时间戳（数字）或 RFC3339/ISO 8601 字符串
	if expVal, ok := claims["exp"]; ok {
		var exp int64
		switch v := expVal.(type) {
		case float64:
			exp = int64(v)
		case int64:
			exp = v
		case string:
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				log.Printf("[JWT] exp 字符串解析失败: %v", err)
				return nil, errors.New("JWT exp 格式错误，应为 Unix 时间戳或 RFC3339/ISO 8601 字符串")
			}
			exp = t.Unix()
		default:
			log.Printf("[JWT] exp 类型错误: %T", expVal)
			return nil, errors.New("JWT exp 格式错误，应为 Unix 时间戳或 RFC3339/ISO 8601 字符串")
		}
		if time.Now().Unix() > exp {
			log.Printf("[JWT] Token 已过期，exp: %d", exp)
			return nil, errors.New("JWT 已过期")
		}
	}

	// 强制校验 tag，service_tag 必须已配置且与 JWT 中的 tag 完全一致
	expectedTag := conf.GetConfig().Auth.ServiceTag
	if expectedTag == "" {
		log.Printf("[JWT] 拒绝：服务端 auth.service_tag 未配置")
		return nil, errors.New("服务端 auth.service_tag 未配置，拒绝所有 JWT 请求")
	}
	tag, _ := claims["tag"].(string)
	if tag != expectedTag {
		log.Printf("[JWT] tag 不匹配，期望: %q，实际: %q", expectedTag, tag)
		return nil, fmt.Errorf("JWT tag 不匹配：期望 %q，实际 %q", expectedTag, tag)
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		log.Printf("[JWT] 解码 signature 失败: %v", err)
		return nil, fmt.Errorf("解码 JWT signature 失败: %w", err)
	}

	signedData := []byte(parts[0] + "." + parts[1])
	hash := crypto.SHA256.New()
	hash.Write(signedData)
	digest := hash.Sum(nil)

	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest, signature); err != nil {
		log.Printf("[JWT] 签名验证失败: %v", err)
		return nil, fmt.Errorf("JWT 签名验证失败: %w", err)
	}

	log.Printf("[JWT] 验签成功")
	return claims, nil
}
