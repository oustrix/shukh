import { Routes, Route } from 'react-router-dom'
import { ROOM_ROUTE } from './routes'
import { Join } from './ui/screens/Join'
import { Room } from './ui/screens/Room'

export function App() {
  return (
    <Routes>
      <Route path="/" element={<Join />} />
      <Route path={ROOM_ROUTE} element={<Room />} />
    </Routes>
  )
}
