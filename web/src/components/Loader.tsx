export default function Loader({ text = '加载中...' }: { text?: string }) {
    return (
        <div className="loader-container">
            <div className="spinner"></div>
            <div className="loader-text">{text}</div>
        </div>
    )
}